package service

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	modelSkillTaskTextMaxBytes = 64 * 1024
	modelSkillRouteMaxPatterns = 64
	modelSkillRouteMaxPatternBytes = 512
	modelSkillRouteMarker = "<!-- sub2api:skill-route:"
	modelSkillRouteV1Prefix = "<!-- sub2api:skill-route:v1 "
)

// modelSkillTaskRoute is deliberately small and data-only. Patterns use Go's
// RE2-compatible regexp engine: lookaround and backreferences are unsupported.
type modelSkillTaskRoute struct {
	ID       string   `json:"id"`
	AnyRegex []string `json:"any_regex"`
	NotRegex []string `json:"not_regex"`
}

// selectSkillTaskText selects only the latest user task in this request. It
// never reads system/developer messages, tool results, assistant output or
// attachment data. A control-only continuation reuses the preceding user task
// in the same request; it does not consult server-side conversation state.
func selectSkillTaskText(root map[string]any, protocol string) string {
	if root == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "responses", "openai":
		if input, ok := root["input"].(string); ok {
			if isSkillTaskContinuation(input) {
				return ""
			}
			return skillTaskTextTail(input)
		}
		return selectSkillTaskFromMessages(root["input"], false)
	case "chat_completions", "chat-completions", "chat", "anthropic":
		return selectSkillTaskFromMessages(root["messages"], false)
	case "gemini":
		return selectSkillTaskFromMessages(root["contents"], true)
	default:
		return ""
	}
}

func selectSkillTaskFromMessages(value any, gemini bool) string {
	messages, ok := value.([]any)
	if !ok {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		if !strings.EqualFold(strings.TrimSpace(role), "user") {
			continue
		}
		// Responses tool-output records are never treated as human messages,
		// even if an unusual client also supplied a role field.
		if !gemini {
			kind, _ := message["type"].(string)
			if kind != "" && kind != "message" {
				continue
			}
		}
		var task string
		var continuation bool
		if gemini {
			task, continuation = skillTaskPartsText(message["parts"], true)
		} else {
			task, continuation = skillTaskPartsText(message["content"], false)
		}
		if continuation {
			continue
		}
		// Empty/image-only latest user messages intentionally stop the search:
		// an old reverse-engineering task must not leak into a new empty task.
		return skillTaskTextTail(task)
	}
	return ""
}

func skillTaskPartsText(value any, gemini bool) (string, bool) {
	if text, ok := value.(string); ok && !gemini {
		return skillTaskTextTail(text), isSkillTaskContinuation(text)
	}
	parts, ok := value.([]any)
	if !ok {
		return "", false
	}
	var result, shortText string
	shortPossible := true
	for _, part := range parts {
		block, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if gemini {
			if _, exists := block["functionCall"]; exists {
				continue
			}
			if _, exists := block["functionResponse"]; exists {
				continue
			}
			if _, exists := block["inlineData"]; exists {
				continue
			}
			if _, exists := block["fileData"]; exists {
				continue
			}
		} else {
			kind, _ := block["type"].(string)
			if kind != "input_text" && kind != "text" {
				continue
			}
		}
		text, ok := block["text"].(string)
		if !ok || text == "" {
			continue
		}
		if shortPossible {
			trimmed := strings.TrimSpace(text)
			if len(trimmed) > 64 || len(shortText)+len(trimmed)+1 > 128 {
				shortPossible = false
			} else {
				shortText += "\n" + trimmed
			}
		}
		if result == "" {
			result = skillTaskTextTail(text)
		} else {
			result = skillTaskTextTail(result + "\n" + skillTaskTextTail(text))
		}
	}
	return result, shortPossible && isSkillTaskContinuation(shortText)
}

func isSkillTaskContinuation(text string) bool {
	// Exact short control turns only. "continue with another task" is a new
	// task, not permission to retain routing from an earlier user message.
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	switch normalized {
	case "继续", "继续分析", "接着", "continue", "go on", "resume", "next step":
		return true
	default:
		return false
	}
}

// skillTaskTextTail caps the match input in bytes without splitting a UTF-8
// rune. Invalid UTF-8 input is replaced, then capped again if replacement grew.
func skillTaskTextTail(text string) string {
	text = rawSkillTaskTail(text)
	text = strings.ToValidUTF8(text, "\uFFFD")
	return rawSkillTaskTail(text)
}

func rawSkillTaskTail(text string) string {
	if len(text) <= modelSkillTaskTextMaxBytes {
		return text
	}
	text = text[len(text)-modelSkillTaskTextMaxBytes:]
	for len(text) > 0 && !utf8.RuneStart(text[0]) {
		text = text[1:]
	}
	return text
}

// routeModelSkillContent applies optional first-line routing metadata. Legacy
// Skills without metadata remain always-on. A malformed marker/JSON/regexp,
// missing positive patterns or empty task skips only this Skill. A selected
// routed Skill returns its body without the metadata line.
func routeModelSkillContent(content, taskText string) (string, bool) {
	first, body, _ := strings.Cut(content, "\n")
	first = strings.TrimSuffix(first, "\r")
	// Accept a UTF-8 BOM and indentation in the metadata line only. Plain
	// legacy content is returned byte-for-byte, without trimming.
	markerLine := strings.TrimSpace(strings.TrimPrefix(first, "\uFEFF"))
	if !strings.HasPrefix(markerLine, modelSkillRouteMarker) {
		return content, true
	}
	if !strings.HasPrefix(markerLine, modelSkillRouteV1Prefix) || !strings.HasSuffix(markerLine, " -->") {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(markerLine, modelSkillRouteV1Prefix), " -->")
	var route modelSkillTaskRoute
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&route); err != nil {
		return "", false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", false
	}
	if len(route.AnyRegex) == 0 || len(route.AnyRegex)+len(route.NotRegex) > modelSkillRouteMaxPatterns {
		return "", false
	}
	patterns := make([]*regexp.Regexp, 0, len(route.AnyRegex)+len(route.NotRegex))
	for _, pattern := range append(append([]string(nil), route.AnyRegex...), route.NotRegex...) {
		if strings.TrimSpace(pattern) == "" || len(pattern) > modelSkillRouteMaxPatternBytes {
			return "", false
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return "", false
		}
		patterns = append(patterns, compiled)
	}
	taskText = skillTaskTextTail(taskText)
	if strings.TrimSpace(taskText) == "" {
		return "", false
	}
	matched := false
	for _, pattern := range patterns[:len(route.AnyRegex)] {
		if pattern.MatchString(taskText) {
			matched = true
			break
		}
	}
	if !matched {
		return "", false
	}
	for _, pattern := range patterns[len(route.AnyRegex):] {
		if pattern.MatchString(taskText) {
			return "", false
		}
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false
	}
	return body, true
}
