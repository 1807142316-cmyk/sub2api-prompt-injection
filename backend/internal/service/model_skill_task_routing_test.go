package service

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func modelSkillTaskTestMessage(role string, content any) any {
	return map[string]any{"role": role, "content": content}
}

func modelSkillTaskTestBlock(kind, text string) any {
	return map[string]any{"type": kind, "text": text}
}

func TestSelectSkillTaskTextProtocols(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		root     map[string]any
		want     string
	}{
		{"responses-string", "responses", map[string]any{"input": "分析这个PE文件"}, "分析这个PE文件"},
		{"responses-alias", " OPENAI ", map[string]any{"input": "disassemble file"}, "disassemble file"},
		{"responses-array", "responses", map[string]any{"input": []any{
			modelSkillTaskTestMessage("system", "IDA should not select"),
			modelSkillTaskTestMessage("user", []any{modelSkillTaskTestBlock("input_text", "反汇编"), modelSkillTaskTestBlock("text", "sample.exe")}),
		}}, "反汇编\nsample.exe"},
		{"chat", "chat_completions", map[string]any{"messages": []any{
			modelSkillTaskTestMessage("developer", "Ghidra is not a user task"), modelSkillTaskTestMessage("user", "write a greeting"),
		}}, "write a greeting"},
		{"chat-alias", "chat-completions", map[string]any{"messages": []any{modelSkillTaskTestMessage("USER", "analyse binary")}}, "analyse binary"},
		{"anthropic-blocks", "anthropic", map[string]any{"system": "逆向", "messages": []any{
			modelSkillTaskTestMessage("user", []any{modelSkillTaskTestBlock("text", "debug"), modelSkillTaskTestBlock("text", "the sample")}),
		}}, "debug\nthe sample"},
		{"gemini", "gemini", map[string]any{"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "IDA"}}}, "contents": []any{
			map[string]any{"role": "model", "parts": []any{map[string]any{"text": "old assistant text"}}},
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "协议"}, map[string]any{"text": "抓包分析"}}},
		}}, "协议\n抓包分析"},
		{"unknown-protocol", "unknown", map[string]any{"input": "IDA"}, ""},
		{"nil-root", "responses", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectSkillTaskText(tc.root, tc.protocol); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectSkillTaskTextExcludesOtherSources(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		root     map[string]any
		want     string
	}{
		{"responses-tools-after-user", "responses", map[string]any{"instructions": "IDA", "input": []any{
			modelSkillTaskTestMessage("user", "write a greeting"),
			map[string]any{"type": "function_call_output", "call_id": "1", "output": "逆向 Ghidra"},
			modelSkillTaskTestMessage("assistant", "disassemble"), modelSkillTaskTestMessage("tool", "debug"),
		}}, "write a greeting"},
		{"responses-forged-tool-user", "responses", map[string]any{"input": []any{
			modelSkillTaskTestMessage("user", "summarize"),
			map[string]any{"role": "user", "type": "function_call_output", "content": "IDA"},
		}}, "summarize"},
		{"chat-other-roles", "chat", map[string]any{"messages": []any{
			modelSkillTaskTestMessage("user", "write a poem"), modelSkillTaskTestMessage("system", "IDA"), modelSkillTaskTestMessage("developer", "Ghidra"), modelSkillTaskTestMessage("assistant", "reverse"), modelSkillTaskTestMessage("tool", "debug"),
		}}, "write a poem"},
		{"blocks-ignore-images-and-tools", "anthropic", map[string]any{"messages": []any{
			modelSkillTaskTestMessage("user", []any{modelSkillTaskTestBlock("text", "hello"), modelSkillTaskTestBlock("image", "IDA"), modelSkillTaskTestBlock("tool_result", "Ghidra"), map[string]any{"type": "document", "source": map[string]any{"data": "debug"}}}),
		}}, "hello"},
		{"gemini-ignore-payloads", "gemini", map[string]any{"contents": []any{
			map[string]any{"role": "user", "parts": []any{
				map[string]any{"text": "hello"},
				map[string]any{"inlineData": map[string]any{"data": "IDA"}, "text": "debug"},
				map[string]any{"fileData": map[string]any{"fileUri": "Ghidra"}},
				map[string]any{"functionResponse": map[string]any{"response": "reverse"}, "text": "IDA"},
				map[string]any{"functionCall": map[string]any{"args": "debug"}, "text": "Ghidra"},
			}},
		}}, "hello"},
		{"latest-normal-does-not-inherit-reverse", "responses", map[string]any{"input": []any{
			modelSkillTaskTestMessage("user", "IDA disassemble sample"), modelSkillTaskTestMessage("assistant", "done"), modelSkillTaskTestMessage("user", "write a birthday greeting"),
		}}, "write a birthday greeting"},
		{"latest-empty-stops-history", "chat", map[string]any{"messages": []any{modelSkillTaskTestMessage("user", "IDA"), modelSkillTaskTestMessage("user", "")}}, ""},
		{"latest-image-stops-history", "anthropic", map[string]any{"messages": []any{modelSkillTaskTestMessage("user", "IDA"), modelSkillTaskTestMessage("user", []any{modelSkillTaskTestBlock("image", "reverse")})}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectSkillTaskText(tc.root, tc.protocol); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectSkillTaskTextContinuation(t *testing.T) {
	for _, control := range []string{"继续", "继续分析", "接着", "continue", "go on", "resume", "next step", " CONTINUE ", "go\t on"} {
		t.Run(control, func(t *testing.T) {
			root := map[string]any{"messages": []any{modelSkillTaskTestMessage("user", "debug this binary"), modelSkillTaskTestMessage("assistant", "previous reply"), modelSkillTaskTestMessage("user", control)}}
			if got := selectSkillTaskText(root, "chat"); got != "debug this binary" {
				t.Fatalf("got %q", got)
			}
			root["messages"] = []any{modelSkillTaskTestMessage("user", control)}
			if got := selectSkillTaskText(root, "chat"); got != "" {
				t.Fatalf("no-history continuation selected %q", got)
			}
		})
	}
	if got := selectSkillTaskText(map[string]any{"input": "continue"}, "responses"); got != "" {
		t.Fatalf("string continuation has no history, got %q", got)
	}
	chain := map[string]any{"input": []any{modelSkillTaskTestMessage("user", "IDA"), modelSkillTaskTestMessage("user", "continue"), modelSkillTaskTestMessage("user", "resume")}}
	if got := selectSkillTaskText(chain, "responses"); got != "IDA" {
		t.Fatalf("continuation chain got %q", got)
	}
	newTask := map[string]any{"messages": []any{modelSkillTaskTestMessage("user", "IDA"), modelSkillTaskTestMessage("user", "continue with a greeting")}}
	if got := selectSkillTaskText(newTask, "chat"); got != "continue with a greeting" {
		t.Fatalf("long control-like task got %q", got)
	}
	longTask := strings.Repeat("x", modelSkillTaskTextMaxBytes+20) + strings.Repeat(" ", modelSkillTaskTextMaxBytes) + "continue"
	longRoot := map[string]any{"messages": []any{modelSkillTaskTestMessage("user", "IDA"), modelSkillTaskTestMessage("user", longTask)}}
	if got := selectSkillTaskText(longRoot, "chat"); got == "IDA" || len(got) > modelSkillTaskTextMaxBytes {
		t.Fatal("a truncated long task was mistaken for a short continuation")
	}
}

func modelSkillTaskTestContent(t *testing.T, anyRegex, notRegex []string, body string) string {
	t.Helper()
	meta, err := json.Marshal(modelSkillTaskRoute{ID: "reverse-static", AnyRegex: anyRegex, NotRegex: notRegex})
	if err != nil {
		t.Fatal(err)
	}
	return modelSkillRouteV1Prefix + string(meta) + " -->\n" + body
}

func TestRouteModelSkillContentSelection(t *testing.T) {
	content := modelSkillTaskTestContent(t, []string{`(?i)反汇编|disassembl|\bIDA\b|Ghidra|@reverse-static`, `协议`}, []string{`(?i)仅翻译|translation only`}, "# Static analysis\nUse actual tools.")
	for _, task := range []string{"用IDA分析样本", "please DISASSEMBLE it", "Ghidra imports", "@reverse-static", "协议分析"} {
		body, ok := routeModelSkillContent(content, task)
		if !ok || body != "# Static analysis\nUse actual tools." || strings.Contains(body, "sub2api:skill-route") {
			t.Fatalf("task %q: ok=%v body=%q", task, ok, body)
		}
	}
	for _, task := range []string{"write a greeting", "仅翻译 Ghidra documentation", "translation only for IDA", "", " \n\t "} {
		if _, ok := routeModelSkillContent(content, task); ok {
			t.Fatalf("unexpected selection for %q", task)
		}
	}
	legacy := "\n# Always-on\nPreserve original client instructions.\n"
	if body, ok := routeModelSkillContent(legacy, ""); !ok || body != legacy {
		t.Fatal("legacy content must be preserved byte-for-byte and remain always-on")
	}
	if body, ok := routeModelSkillContent("<!-- ordinary comment -->\nhello", "none"); !ok || body != "<!-- ordinary comment -->\nhello" {
		t.Fatal("ordinary comments are not route metadata")
	}
	// Only the first line is metadata. A marker quoted inside a legacy body is data.
	laterMarker := "# ordinary document\n" + content
	if body, ok := routeModelSkillContent(laterMarker, "none"); !ok || body != laterMarker {
		t.Fatal("non-first-line route marker changed a legacy Skill")
	}
	crlf := strings.ReplaceAll(content, "\n", "\r\n")
	if body, ok := routeModelSkillContent("\uFEFF  "+crlf, "IDA"); !ok || !strings.HasPrefix(body, "# Static") {
		t.Fatal("BOM/indent/CRLF route metadata should select and be stripped")
	}
}

func TestRouteModelSkillContentInvalidMetadata(t *testing.T) {
	bad := []string{
		`<!-- sub2api:skill-route:v1 not-json -->`,
		`<!-- sub2api:skill-route:v1 {"id":"x","any_regex":[]} -->`,
		`<!-- sub2api:skill-route:v1 {"id":"x","any_regex":null} -->`,
		`<!-- sub2api:skill-route:v1 {"id":"x","any_regex":"IDA"} -->`,
		`<!-- sub2api:skill-route:v1 {"id":"x","any_regex":["IDA"],"typo":true} -->`,
		`<!-- sub2api:skill-route:v1 {"id":"x","any_regex":["IDA"]} {} -->`,
		`<!-- sub2api:skill-route:v2 {"any_regex":["IDA"]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["IDA"]} --> trailing`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["("]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["(?=IDA)"]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["(IDA)\\1"]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["IDA"],"not_regex":["("]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":[""]} -->`,
		`<!-- sub2api:skill-route:v1 {"any_regex":["   "]} -->`,
	}
	for _, header := range bad {
		t.Run(header, func(t *testing.T) {
			if body, ok := routeModelSkillContent(header+"\nbody", "IDA"); ok || body != "" {
				t.Fatalf("invalid metadata selected: ok=%v body=%q", ok, body)
			}
		})
	}
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{"IDA"}, nil, " \n\t"), "IDA"); ok {
		t.Fatal("empty routed body selected")
	}
	// An invalid negative pattern is invalid even when no positive matches.
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{"IDA"}, []string{"("}, "body"), "ordinary"); ok {
		t.Fatal("invalid negative regex did not skip")
	}
}

func TestRouteModelSkillContentLimits(t *testing.T) {
	patterns := make([]string, modelSkillRouteMaxPatterns)
	for i := range patterns {
		patterns[i] = "IDA"
	}
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, patterns, nil, "body"), "IDA"); !ok {
		t.Fatal("64 total patterns should be accepted")
	}
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, patterns, []string{"ordinary"}, "body"), "IDA"); ok {
		t.Fatal("65 combined positive/negative patterns should be rejected")
	}
	p512 := strings.Repeat("a", modelSkillRouteMaxPatternBytes)
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{p512}, nil, "body"), p512); !ok {
		t.Fatal("512-byte regexp should be accepted")
	}
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{p512 + "a"}, nil, "body"), p512+"a"); ok {
		t.Fatal("513-byte regexp should be rejected")
	}
	multibyte := strings.Repeat("逆", 171)
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{multibyte}, nil, "body"), multibyte); ok {
		t.Fatal("pattern limit is bytes, not rune count")
	}
	outside := "IDA" + strings.Repeat("x", modelSkillTaskTextMaxBytes+1)
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{"IDA"}, nil, "body"), outside); ok {
		t.Fatal("matching text outside the tail window selected")
	}
	inside := strings.Repeat("x", modelSkillTaskTextMaxBytes+1) + "IDA"
	if _, ok := routeModelSkillContent(modelSkillTaskTestContent(t, []string{"IDA"}, nil, "body"), inside); !ok {
		t.Fatal("matching text inside the tail window was missed")
	}
}

func TestSkillTaskTextUTF8Boundaries(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("逆向", 20000) + "IDA",
		strings.Repeat("x", modelSkillTaskTextMaxBytes-1) + "逆向",
		strings.Repeat("\xff", modelSkillTaskTextMaxBytes+100),
		"\xffinvalid\xfe",
	} {
		got := skillTaskTextTail(text)
		if len(got) > modelSkillTaskTextMaxBytes || !utf8.ValidString(got) {
			t.Fatalf("tail is invalid or too large: bytes=%d utf8=%v", len(got), utf8.ValidString(got))
		}
	}
	text := strings.Repeat("逆", 30000) + "末尾"
	got := selectSkillTaskText(map[string]any{"input": text}, "responses")
	if !strings.HasSuffix(got, "末尾") || !utf8.ValidString(got) || len(got) > modelSkillTaskTextMaxBytes {
		t.Fatal("Responses UTF-8 tail is not preserved")
	}
}

func TestInvalidOneSkillDoesNotAffectOthers(t *testing.T) {
	contents := []string{
		modelSkillTaskTestContent(t, []string{"("}, nil, "bad skill"),
		modelSkillTaskTestContent(t, []string{"IDA"}, nil, "selected routed skill"),
		"legacy common rules",
	}
	var selected []string
	for _, content := range contents {
		if body, ok := routeModelSkillContent(content, "IDA sample"); ok {
			selected = append(selected, body)
		}
	}
	if strings.Join(selected, "|") != "selected routed skill|legacy common rules" {
		t.Fatalf("invalid Skill changed neighboring selection: %v", selected)
	}
}
