package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyInstructionLayers       = "instruction_layers"
	MaxInstructionLayersConfigBytes   = 2 * 1024 * 1024
	maxInstructionLayerTextBytes      = 128 * 1024
	maxInstructionLayerProfiles       = 128
	instructionLayersPrefixMarker     = "<!-- sub2api:instruction-layers:v1 -->"
	instructionLayersSuffixMarker     = "<!-- sub2api:instruction-layers-suffix:v1 -->"
	instructionLayersPrefixJSONMarker = `\u003c!-- sub2api:instruction-layers:v1 --\u003e`
	instructionLayersSuffixJSONMarker = `\u003c!-- sub2api:instruction-layers-suffix:v1 --\u003e`
)

// InstructionLayerProfile replaces, rather than merges with, the less-specific
// profile. An explicitly disabled profile therefore blocks scope inheritance.
// Text is never trimmed: leading/trailing whitespace remains administrator data.
type InstructionLayerProfile struct {
	Enabled       bool              `json:"enabled"`
	Prefix        string            `json:"prefix"`
	Suffix        string            `json:"suffix"`
	ModelSuffixes map[string]string `json:"model_suffixes"`
}

// InstructionLayersConfig is an opt-in administrator setting. Scope selection is
// authenticated API Key ID > authenticated user ID > authenticated group ID >
// default. A disabled group is a hard opt-out from all server-side injection.
type InstructionLayersConfig struct {
	Enabled bool                               `json:"enabled"`
	Default InstructionLayerProfile            `json:"default"`
	Groups  map[string]InstructionLayerProfile `json:"groups"`
	Users   map[string]InstructionLayerProfile `json:"users"`
	APIKeys map[string]InstructionLayerProfile `json:"api_keys"`
}

func emptyInstructionLayersConfig() InstructionLayersConfig {
	return InstructionLayersConfig{
		Default: InstructionLayerProfile{ModelSuffixes: map[string]string{}},
		Groups:  map[string]InstructionLayerProfile{},
		Users:   map[string]InstructionLayerProfile{}, APIKeys: map[string]InstructionLayerProfile{},
	}
}

func instructionLayersValidationError(message string) error {
	return infraerrors.BadRequest("INSTRUCTION_LAYERS_INVALID", message)
}

func validateInstructionLayerProfile(profile InstructionLayerProfile, location string) (InstructionLayerProfile, error) {
	validateText := func(value, field string) error {
		if !utf8.ValidString(value) {
			return instructionLayersValidationError(field + " must contain valid UTF-8")
		}
		if len(value) > maxInstructionLayerTextBytes {
			return instructionLayersValidationError(field + " exceeds 128 KiB")
		}
		return nil
	}
	if err := validateText(profile.Prefix, location+".prefix"); err != nil {
		return InstructionLayerProfile{}, err
	}
	if err := validateText(profile.Suffix, location+".suffix"); err != nil {
		return InstructionLayerProfile{}, err
	}
	models := make(map[string]string, len(profile.ModelSuffixes))
	for model, text := range profile.ModelSuffixes {
		if !utf8.ValidString(model) || strings.TrimSpace(model) == "" || utf8.RuneCountInString(model) > 200 {
			return InstructionLayerProfile{}, instructionLayersValidationError(location + ".model_suffixes requires non-empty model keys of at most 200 characters")
		}
		if err := validateText(text, location+".model_suffixes["+model+"]"); err != nil {
			return InstructionLayerProfile{}, err
		}
		models[model] = text
	}
	profile.ModelSuffixes = models
	return profile, nil
}

func normalizeInstructionLayers(input InstructionLayersConfig) (InstructionLayersConfig, error) {
	if 1+len(input.Groups)+len(input.Users)+len(input.APIKeys) > maxInstructionLayerProfiles {
		return InstructionLayersConfig{}, instructionLayersValidationError("instruction layers exceed 128 total profiles")
	}
	config := emptyInstructionLayersConfig()
	config.Enabled = input.Enabled
	var err error
	config.Default, err = validateInstructionLayerProfile(input.Default, "default")
	if err != nil {
		return InstructionLayersConfig{}, err
	}
	for _, scope := range []struct {
		name   string
		input  map[string]InstructionLayerProfile
		output map[string]InstructionLayerProfile
	}{{"groups", input.Groups, config.Groups}, {"users", input.Users, config.Users}, {"api_keys", input.APIKeys, config.APIKeys}} {
		for id, profile := range scope.input {
			// Canonical positive decimal IDs correspond to the int64 authentication
			// IDs. Reject signs, zero, leading zeros and integer overflow.
			parsed, parseErr := strconv.ParseInt(id, 10, 64)
			if parseErr != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != id {
				return InstructionLayersConfig{}, instructionLayersValidationError(scope.name + " requires canonical positive integer IDs")
			}
			clean, profileErr := validateInstructionLayerProfile(profile, scope.name+"["+id+"]")
			if profileErr != nil {
				return InstructionLayersConfig{}, profileErr
			}
			scope.output[id] = clean
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return InstructionLayersConfig{}, err
	}
	if len(data) > MaxInstructionLayersConfigBytes {
		return InstructionLayersConfig{}, instructionLayersValidationError("instruction layers exceed 2 MiB")
	}
	return config, nil
}

func (s *SettingService) GetInstructionLayers(ctx context.Context) (InstructionLayersConfig, error) {
	if s == nil || s.settingRepo == nil {
		return emptyInstructionLayersConfig(), nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyInstructionLayers)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return emptyInstructionLayersConfig(), nil
		}
		return InstructionLayersConfig{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return emptyInstructionLayersConfig(), nil
	}
	if len(raw) > MaxInstructionLayersConfigBytes {
		return InstructionLayersConfig{}, instructionLayersValidationError("stored instruction layers exceed 2 MiB")
	}
	var config InstructionLayersConfig
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return InstructionLayersConfig{}, fmt.Errorf("decode instruction layers: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return InstructionLayersConfig{}, instructionLayersValidationError("stored instruction layers contain trailing JSON")
	}
	return normalizeInstructionLayers(config)
}

func (s *SettingService) UpdateInstructionLayers(ctx context.Context, input InstructionLayersConfig) (InstructionLayersConfig, error) {
	config, err := normalizeInstructionLayers(input)
	if err != nil {
		return InstructionLayersConfig{}, err
	}
	if s == nil || s.settingRepo == nil {
		return config, nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return InstructionLayersConfig{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyInstructionLayers, string(data)); err != nil {
		return InstructionLayersConfig{}, err
	}
	return config, nil
}

func instructionLayerGroupProfile(ctx context.Context, config InstructionLayersConfig) (InstructionLayerProfile, bool) {
	groupID, ok := ctx.Value(ctxkey.AuthenticatedGroupID).(int64)
	if !ok || groupID <= 0 {
		return InstructionLayerProfile{}, false
	}
	profile, exists := config.Groups[strconv.FormatInt(groupID, 10)]
	return profile, exists
}

// A group opt-out is independent of the master layer switch and cannot be
// overridden by a user or client profile.
func instructionLayerGroupDisabled(ctx context.Context, config InstructionLayersConfig) bool {
	profile, exists := instructionLayerGroupProfile(ctx, config)
	return exists && !profile.Enabled
}

func resolveInstructionLayerText(ctx context.Context, config InstructionLayersConfig, requestedModel string) (string, string) {
	if !config.Enabled || instructionLayerGroupDisabled(ctx, config) {
		return "", ""
	}
	profile := config.Default
	if candidate, exists := instructionLayerGroupProfile(ctx, config); exists {
		profile = candidate
	}
	if userID, ok := ctx.Value(ctxkey.UserID).(int64); ok && userID > 0 {
		if candidate, exists := config.Users[strconv.FormatInt(userID, 10)]; exists {
			profile = candidate
		}
	}
	if apiKeyID, ok := ctx.Value(ctxkey.APIKeyID).(int64); ok && apiKeyID > 0 {
		if candidate, exists := config.APIKeys[strconv.FormatInt(apiKeyID, 10)]; exists {
			profile = candidate
		}
	}
	if !profile.Enabled {
		return "", ""
	}
	if suffix, exists := profile.ModelSuffixes[requestedModel]; exists {
		return profile.Prefix, suffix
	}
	if suffix, exists := profile.ModelSuffixes["*"]; exists {
		return profile.Prefix, suffix
	}
	return profile.Prefix, profile.Suffix
}

// instructionLayerValue preserves all original text bytes or content blocks.
// Metadata text blocks are independent of the client's pre-existing blocks.
func instructionLayerValue(existing any, prefix, suffix, skills string, location string) (any, error) {
	before, after := "", ""
	if prefix != "" {
		before = instructionLayersPrefixMarker + "\n" + prefix
	}
	if suffix != "" {
		after = instructionLayersSuffixMarker + "\n" + suffix
	}
	if existing == nil {
		existing = ""
	}
	switch value := existing.(type) {
	case string:
		segments := make([]string, 0, 4)
		for _, segment := range []string{before, value, after, skills} {
			if segment != "" {
				segments = append(segments, segment)
			}
		}
		return strings.Join(segments, "\n\n"), nil
	case []any:
		blocks := make([]any, 0, len(value)+3)
		if before != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": before})
		}
		for _, block := range value {
			if _, ok := block.(map[string]any); !ok {
				return nil, fmt.Errorf("%s must contain content-block objects", location)
			}
			blocks = append(blocks, block)
		}
		if after != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": after})
		}
		if skills != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": skills})
		}
		return blocks, nil
	default:
		return nil, fmt.Errorf("%s must be a string or content-block array", location)
	}
}

// applyInstructionLayers wraps only the protocol's system/instructions field.
// User messages and input are untouched. The caller decodes and marshals once.
func applyInstructionLayers(root map[string]any, protocol, prefix, suffix, skills string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "responses", "openai":
		value, err := instructionLayerValue(root["instructions"], prefix, suffix, skills, "responses instructions")
		if err != nil {
			return false, err
		}
		root["instructions"] = value
	case "anthropic":
		value, err := instructionLayerValue(root["system"], prefix, suffix, skills, "anthropic system")
		if err != nil {
			return false, err
		}
		root["system"] = value
	case "chat_completions", "chat-completions", "chat":
		var messages []any
		if root["messages"] != nil {
			var ok bool
			messages, ok = root["messages"].([]any)
			if !ok {
				return false, fmt.Errorf("chat messages must be an array")
			}
		}
		var selected map[string]any
		for _, item := range messages {
			message, ok := item.(map[string]any)
			if !ok {
				return false, fmt.Errorf("chat messages must contain message objects")
			}
			role, _ := message["role"].(string)
			if selected == nil && (strings.EqualFold(role, "system") || strings.EqualFold(role, "developer")) {
				selected = message
			}
		}
		if selected == nil {
			value, err := instructionLayerValue(nil, prefix, suffix, skills, "chat system content")
			if err != nil {
				return false, err
			}
			root["messages"] = append([]any{map[string]any{"role": "system", "content": value}}, messages...)
		} else {
			value, err := instructionLayerValue(selected["content"], prefix, suffix, skills, "chat system content")
			if err != nil {
				return false, err
			}
			selected["content"] = value
		}
	case "gemini":
		var system map[string]any
		if root["systemInstruction"] != nil {
			var ok bool
			system, ok = root["systemInstruction"].(map[string]any)
			if !ok {
				return false, fmt.Errorf("gemini systemInstruction must be an object")
			}
		} else {
			system = map[string]any{}
		}
		var parts []any
		if system["parts"] != nil {
			var ok bool
			parts, ok = system["parts"].([]any)
			if !ok {
				return false, fmt.Errorf("gemini systemInstruction.parts must be an array")
			}
		}
		wrapped := make([]any, 0, len(parts)+3)
		if prefix != "" {
			wrapped = append(wrapped, map[string]any{"text": instructionLayersPrefixMarker + "\n" + prefix})
		}
		for _, part := range parts {
			if _, ok := part.(map[string]any); !ok {
				return false, fmt.Errorf("gemini systemInstruction.parts must contain objects")
			}
			wrapped = append(wrapped, part)
		}
		if suffix != "" {
			wrapped = append(wrapped, map[string]any{"text": instructionLayersSuffixMarker + "\n" + suffix})
		}
		if skills != "" {
			wrapped = append(wrapped, map[string]any{"text": skills})
		}
		system["parts"] = wrapped
		root["systemInstruction"] = system
	default:
		return false, nil
	}
	return true, nil
}
