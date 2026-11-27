//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type instructionLayersSettingRepo struct {
	SettingRepository
	values   map[string]string
	reads    []string
	writes   int
	setError error
}

func (r *instructionLayersSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.reads = append(r.reads, key)
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}
func (r *instructionLayersSettingRepo) Set(_ context.Context, key, value string) error {
	r.writes++
	if r.setError != nil {
		return r.setError
	}
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}
func layerJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func layerRoot(t *testing.T, value []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func layerFixtureConfig() InstructionLayersConfig {
	return InstructionLayersConfig{Enabled: true, Default: InstructionLayerProfile{Enabled: true, Prefix: "  PREFIX_FIXTURE\n\t", Suffix: "\nSUFFIX_FIXTURE  ", ModelSuffixes: map[string]string{}}, Users: map[string]InstructionLayerProfile{}, APIKeys: map[string]InstructionLayerProfile{}}
}
func layerFixtureService(t *testing.T, config InstructionLayersConfig, includeSkill bool) (*SettingService, *instructionLayersSettingRepo) {
	t.Helper()
	values := map[string]string{SettingKeyInstructionLayers: string(layerJSON(t, config))}
	rules := ModelSkillRuleConfig{ModelSkillIDs: map[string][]string{}}
	if includeSkill {
		rules.DefaultSkillIDs = []string{"skill-fixture"}
		values[SettingKeyModelSkills] = string(layerJSON(t, []ModelSkill{{ID: "skill-fixture", Name: "FIXTURE_SKILL", Content: `<!-- sub2api:skill-route:v1 {"id":"fixture","any_regex":["reverse"],"not_regex":[]} -->` + "\nSELECTED_SKILL_FIXTURE"}}))
	}
	values[SettingKeyGlobalModelSkillRules] = string(layerJSON(t, rules))
	repo := &instructionLayersSettingRepo{values: values}
	return &SettingService{settingRepo: repo}, repo
}
func layerText(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		var parts []string
		for _, item := range content {
			if block, ok := item.(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
func layerAssertOrder(t *testing.T, text string, tokens ...string) {
	t.Helper()
	at := -1
	for _, token := range tokens {
		next := strings.Index(text, token)
		if next < 0 || next <= at {
			t.Fatalf("token %q missing or out of order in %q", token, text)
		}
		at = next
	}
}

func TestInstructionLayersConfigurationRoundTrip(t *testing.T) {
	repo := &instructionLayersSettingRepo{values: map[string]string{}}
	settings := &SettingService{settingRepo: repo}
	empty, err := settings.GetInstructionLayers(context.Background())
	if err != nil || empty.Enabled || empty.Default.Enabled || empty.Users == nil || empty.APIKeys == nil {
		t.Fatalf("missing setting must be off with objects: %#v %v", empty, err)
	}
	config := layerFixtureConfig()
	config.Users["1"] = InstructionLayerProfile{Enabled: true, Prefix: "\n USER_PREFIX \n", ModelSuffixes: map[string]string{"gpt-test": "\tMODEL_SUFFIX \n"}}
	got, err := settings.UpdateInstructionLayers(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.GetInstructionLayers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, loaded) || loaded.Default.Prefix != config.Default.Prefix || loaded.Default.Suffix != config.Default.Suffix || loaded.Users["1"].Prefix != config.Users["1"].Prefix {
		t.Fatal("configuration whitespace or values changed")
	}
	config.Default.ModelSuffixes["changed"] = "late mutation"
	if _, exists := got.Default.ModelSuffixes["changed"]; exists {
		t.Fatal("configuration aliases caller map")
	}
	if repo.writes != 1 {
		t.Fatalf("writes=%d", repo.writes)
	}
	failure := errors.New("fixture write failure")
	repo.setError = failure
	if _, err := settings.UpdateInstructionLayers(context.Background(), layerFixtureConfig()); !errors.Is(err, failure) {
		t.Fatalf("write error lost: %v", err)
	}
}

func TestInstructionLayersRejectInvalidConfigurationBeforeWrite(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*InstructionLayersConfig)
	}{
		{"prefix-size", func(c *InstructionLayersConfig) {
			c.Default.Prefix = strings.Repeat("p", maxInstructionLayerTextBytes+1)
		}},
		{"suffix-size", func(c *InstructionLayersConfig) {
			c.Default.Suffix = strings.Repeat("s", maxInstructionLayerTextBytes+1)
		}},
		{"model-text-size", func(c *InstructionLayersConfig) {
			c.Default.ModelSuffixes["model"] = strings.Repeat("x", maxInstructionLayerTextBytes+1)
		}},
		{"model-key-empty", func(c *InstructionLayersConfig) { c.Default.ModelSuffixes[" \t"] = "x" }},
		{"model-key-length", func(c *InstructionLayersConfig) { c.Default.ModelSuffixes[strings.Repeat("模", 201)] = "x" }},
		{"invalid-utf8", func(c *InstructionLayersConfig) { c.Default.Prefix = string([]byte{0xff}) }},
		{"invalid-id-zero", func(c *InstructionLayersConfig) { c.Users["0"] = InstructionLayerProfile{} }},
		{"invalid-id-negative", func(c *InstructionLayersConfig) { c.APIKeys["-1"] = InstructionLayerProfile{} }},
		{"invalid-id-leading-zero", func(c *InstructionLayersConfig) { c.Users["01"] = InstructionLayerProfile{} }},
		{"invalid-id-sign", func(c *InstructionLayersConfig) { c.APIKeys["+1"] = InstructionLayerProfile{} }},
		{"invalid-id-overflow", func(c *InstructionLayersConfig) { c.APIKeys["9223372036854775808"] = InstructionLayerProfile{} }},
		{"profiles-limit", func(c *InstructionLayersConfig) {
			for i := 1; i <= 128; i++ {
				c.Users[fmt.Sprint(i)] = InstructionLayerProfile{}
			}
		}},
		{"config-limit", func(c *InstructionLayersConfig) {
			for i := 1; i <= 17; i++ {
				c.Users[fmt.Sprint(i)] = InstructionLayerProfile{Prefix: strings.Repeat("x", maxInstructionLayerTextBytes)}
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := layerFixtureConfig()
			test.mutate(&config)
			repo := &instructionLayersSettingRepo{values: map[string]string{SettingKeyInstructionLayers: "ORIGINAL_VALUE"}}
			settings := &SettingService{settingRepo: repo}
			if _, err := settings.UpdateInstructionLayers(context.Background(), config); err == nil {
				t.Fatal("expected invalid config error")
			}
			if repo.writes != 0 || repo.values[SettingKeyInstructionLayers] != "ORIGINAL_VALUE" {
				t.Fatal("invalid config mutated settings")
			}
		})
	}
	config := layerFixtureConfig()
	config.Default.Prefix = strings.Repeat("x", maxInstructionLayerTextBytes)
	config.Default.ModelSuffixes[strings.Repeat("模", 200)] = ""
	for i := 1; i <= 127; i++ {
		config.Users[fmt.Sprint(i)] = InstructionLayerProfile{}
	}
	if _, err := normalizeInstructionLayers(config); err != nil {
		t.Fatalf("inclusive limits rejected: %v", err)
	}
}

func TestInstructionLayersStoredConfigurationErrors(t *testing.T) {
	for _, raw := range []string{"{", `{"enabled":true,"unknown":1}`, `{} {}`, strings.Repeat(" ", MaxInstructionLayersConfigBytes) + "{}", `{"enabled":true,"users":{"0":{}}}`} {
		repo := &instructionLayersSettingRepo{values: map[string]string{SettingKeyInstructionLayers: raw}}
		settings := &SettingService{settingRepo: repo}
		if _, err := settings.GetInstructionLayers(context.Background()); err == nil {
			t.Fatal("invalid stored config accepted")
		}
		if repo.writes != 0 {
			t.Fatal("GET must not repair or write config")
		}
	}
}

func TestInstructionLayersScopePriorityAndModelFallback(t *testing.T) {
	config := layerFixtureConfig()
	config.Default = InstructionLayerProfile{Enabled: true, Prefix: "DEFAULT_PREFIX", Suffix: "DEFAULT_SUFFIX", ModelSuffixes: map[string]string{"gpt-test": "DEFAULT_MODEL"}}
	config.Users["1"] = InstructionLayerProfile{Enabled: true, Prefix: "USER_PREFIX", Suffix: "USER_SUFFIX", ModelSuffixes: map[string]string{"gpt-test": "USER_MODEL", "*": "USER_STAR", "empty-model": ""}}
	config.APIKeys["2"] = InstructionLayerProfile{Enabled: true, Prefix: "KEY_PREFIX", Suffix: "KEY_SUFFIX", ModelSuffixes: map[string]string{"gpt-test": "KEY_MODEL"}}
	base := context.Background()
	user := context.WithValue(base, ctxkey.UserID, int64(1))
	key := context.WithValue(user, ctxkey.APIKeyID, int64(2))
	cases := []struct {
		name                  string
		ctx                   context.Context
		model, prefix, suffix string
	}{
		{"default", base, "gpt-test", "DEFAULT_PREFIX", "DEFAULT_MODEL"},
		{"user", user, "gpt-test", "USER_PREFIX", "USER_MODEL"},
		{"key", key, "gpt-test", "KEY_PREFIX", "KEY_MODEL"},
		{"user-wildcard", user, "other", "USER_PREFIX", "USER_STAR"},
		{"key-whole-replace", key, "other", "KEY_PREFIX", "KEY_SUFFIX"},
		{"explicit-empty-model", user, "empty-model", "USER_PREFIX", ""},
		{"key-without-user", context.WithValue(base, ctxkey.APIKeyID, int64(2)), "other", "KEY_PREFIX", "KEY_SUFFIX"},
		{"context-string-is-not-auth-id", context.WithValue(base, ctxkey.UserID, "1"), "gpt-test", "DEFAULT_PREFIX", "DEFAULT_MODEL"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p, s := resolveInstructionLayerText(test.ctx, config, test.model)
			if p != test.prefix || s != test.suffix {
				t.Fatalf("got %q/%q expected %q/%q", p, s, test.prefix, test.suffix)
			}
		})
	}
	config.APIKeys["2"] = InstructionLayerProfile{Enabled: false}
	p, s := resolveInstructionLayerText(key, config, "gpt-test")
	if p != "" || s != "" {
		t.Fatal("disabled key must block user/default inheritance")
	}
	delete(config.APIKeys, "2")
	config.Users["1"] = InstructionLayerProfile{Enabled: false}
	p, s = resolveInstructionLayerText(user, config, "gpt-test")
	if p != "" || s != "" {
		t.Fatal("disabled user must block default inheritance")
	}
	config.Enabled = false
	p, s = resolveInstructionLayerText(base, config, "gpt-test")
	if p != "" || s != "" {
		t.Fatal("global off must disable every profile")
	}
}

func TestInstructionLayersFourProtocolStringsAndArrays(t *testing.T) {
	client := " \tCLIENT_BYTES_FIXTURE\n  "
	image := map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "IMAGE_FIXTURE"}}
	for _, protocol := range []string{"responses", "anthropic", "chat_completions", "gemini"} {
		for _, array := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/array=%v", protocol, array), func(t *testing.T) {
				if protocol == "gemini" && !array {
					return
				}
				config := layerFixtureConfig()
				settings, _ := layerFixtureService(t, config, true)
				root := map[string]any{"model": "fixture-model"}
				originalBlocks := []any{map[string]any{"type": "text", "text": client, "cache_control": map[string]any{"type": "ephemeral"}}, image}
				var original any = client
				if array {
					original = originalBlocks
				}
				user := map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "reverse task", "extra": "USER_EXTRA"}, image}}
				switch protocol {
				case "responses":
					root["instructions"] = original
					root["input"] = []any{user}
				case "anthropic":
					root["system"] = original
					root["messages"] = []any{user}
				case "chat_completions":
					root["messages"] = []any{map[string]any{"role": "developer", "content": original, "name": "CLIENT_ROLE_NAME"}, user, map[string]any{"role": "system", "content": "SECOND_SYSTEM_UNCHANGED"}}
				case "gemini":
					root["systemInstruction"] = map[string]any{"role": "system", "custom": "RETAIN_SYSTEM_METADATA", "parts": []any{map[string]any{"text": client}, map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "IMAGE_FIXTURE"}}}}
					root["contents"] = []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "reverse task"}}}}
				}
				body := layerJSON(t, root)
				before := layerRoot(t, body)
				got, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, protocol, "fixture-model", body)
				if err != nil {
					t.Fatal(err)
				}
				after := layerRoot(t, got)
				var instructions any
				switch protocol {
				case "responses":
					instructions = after["instructions"]
					if !reflect.DeepEqual(before["input"], after["input"]) {
						t.Fatal("user input changed")
					}
				case "anthropic":
					instructions = after["system"]
					if !reflect.DeepEqual(before["messages"], after["messages"]) {
						t.Fatal("user messages changed")
					}
				case "chat_completions":
					messages := after["messages"].([]any)
					first := messages[0].(map[string]any)
					instructions = first["content"]
					if first["role"] != "developer" || first["name"] != "CLIENT_ROLE_NAME" {
						t.Fatal("client role/metadata changed")
					}
					if !reflect.DeepEqual(before["messages"].([]any)[1:], messages[1:]) {
						t.Fatal("other messages changed")
					}
				case "gemini":
					system := after["systemInstruction"].(map[string]any)
					instructions = system["parts"]
					if system["role"] != "system" || system["custom"] != "RETAIN_SYSTEM_METADATA" {
						t.Fatal("system metadata changed")
					}
					if !reflect.DeepEqual(before["contents"], after["contents"]) {
						t.Fatal("user contents changed")
					}
				}
				text := layerText(instructions)
				layerAssertOrder(t, text, config.Default.Prefix, client, config.Default.Suffix, "SELECTED_SKILL_FIXTURE")
				if strings.Count(text, modelSkillsInjectionMarker) != 1 || strings.Contains(text, "sub2api:skill-route:") {
					t.Fatal("Skill marker or routing metadata incorrect")
				}
				if array {
					blocks := instructions.([]any)
					var old []any
					switch protocol {
					case "responses":
						old = before["instructions"].([]any)
					case "anthropic":
						old = before["system"].([]any)
					case "chat_completions":
						old = before["messages"].([]any)[0].(map[string]any)["content"].([]any)
					case "gemini":
						old = before["systemInstruction"].(map[string]any)["parts"].([]any)
					}
					if !reflect.DeepEqual(old, blocks[1:1+len(old)]) {
						t.Fatal("client content blocks changed")
					}
					if len(blocks) != len(old)+3 {
						t.Fatalf("prefix, suffix and Skills should be independent blocks: %d", len(blocks))
					}
				}
				again, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, protocol, "fixture-model", got)
				if err != nil || string(again) != string(got) {
					t.Fatalf("reapplication changed body: %v", err)
				}
			})
		}
	}
}

func TestInstructionLayersNoSkillsEmptyClientsAndStringMarkers(t *testing.T) {
	for _, protocol := range []string{"responses", "anthropic", "chat", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			config := layerFixtureConfig()
			settings, repo := layerFixtureService(t, config, false)
			root := map[string]any{"model": "fixture", "input": "USER_VALUE", "messages": []any{map[string]any{"role": "user", "content": "USER_VALUE"}}, "contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "USER_VALUE"}}}}}
			body := layerJSON(t, root)
			got, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, protocol, "fixture", body)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) == string(body) {
				t.Fatal("empty Skill candidate pool skipped instruction layers")
			}
			if strings.Contains(string(got), modelSkillsInjectionJSONMarker) {
				t.Fatal("absent Skills marker appeared")
			}
			for _, read := range repo.reads {
				if read == SettingKeyModelSkills {
					t.Fatal("empty pool should not read Skill contents")
				}
			}
			for _, candidate := range [][]byte{got, []byte(strings.ReplaceAll(strings.ReplaceAll(string(got), `\u003c`, "<"), `\u003e`, ">"))} {
				repo.reads = nil
				again, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, protocol, "fixture", candidate)
				if err != nil || string(again) != string(candidate) || len(repo.reads) != 0 {
					t.Fatal("plain/escaped layer markers must dedupe before settings reads")
				}
			}
		})
	}
	for _, suffixOnly := range []bool{false, true} {
		config := layerFixtureConfig()
		if suffixOnly {
			config.Default.Prefix = ""
		} else {
			config.Default.Suffix = ""
		}
		settings, _ := layerFixtureService(t, config, false)
		body := []byte(`{"instructions":"","input":"normal"}`)
		got, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, "responses", "fixture", body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "FIXTURE") {
			t.Fatal("single-sided layer missing")
		}
		again, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, "responses", "fixture", got)
		if err != nil || string(again) != string(got) {
			t.Fatal("single-sided dedupe failed")
		}
	}
}

func TestInstructionLayersDisabledAndUnselectedSkillsPreserveBody(t *testing.T) {
	for _, name := range []string{"global-off", "default-off", "empty", "scope-disabled"} {
		t.Run(name, func(t *testing.T) {
			config := layerFixtureConfig()
			ctx := context.Background()
			switch name {
			case "global-off":
				config.Enabled = false
			case "default-off":
				config.Default.Enabled = false
			case "empty":
				config.Default.Prefix = ""
				config.Default.Suffix = ""
			case "scope-disabled":
				config.Users["1"] = InstructionLayerProfile{Enabled: false}
				ctx = context.WithValue(ctx, ctxkey.UserID, int64(1))
			}
			settings, _ := layerFixtureService(t, config, true)
			body := []byte("{ \n\"instructions\":\" CLIENT \", \"input\":\"ordinary task\" }")
			got, err := settings.ApplyGroupDefaultSkills(ctx, nil, "responses", "fixture", body)
			if err != nil || string(got) != string(body) {
				t.Fatalf("inactive config/unselected Skill mutated body: %v", err)
			}
		})
	}
	settings, _ := layerFixtureService(t, emptyInstructionLayersConfig(), false)
	body := []byte("not-json-but-unused")
	got, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, "responses", "fixture", body)
	if err != nil || string(got) != string(body) {
		t.Fatal("inactive empty configuration should not parse unused body")
	}
}

func TestInstructionLayersRejectInvalidClientInstructionFormats(t *testing.T) {
	cases := []struct{ protocol, raw string }{
		{"responses", `{"instructions":123,"input":"normal"}`}, {"responses", `{"instructions":["invalid"],"input":"normal"}`}, {"responses", `null`}, {"responses", `{`},
		{"anthropic", `{"system":{"unexpected":"object"},"messages":[]}`},
		{"chat", `{"messages":"invalid"}`}, {"chat", `{"messages":[false]}`}, {"chat", `{"messages":[{"role":"developer","content":false}]}`},
		{"gemini", `{"systemInstruction":"invalid"}`}, {"gemini", `{"systemInstruction":{"parts":"invalid"}}`}, {"gemini", `{"systemInstruction":{"parts":[false]}}`},
	}
	for _, test := range cases {
		t.Run(test.protocol+"/"+test.raw, func(t *testing.T) {
			settings, _ := layerFixtureService(t, layerFixtureConfig(), false)
			body := []byte(test.raw)
			got, err := settings.ApplyGroupDefaultSkills(context.Background(), nil, test.protocol, "fixture", body)
			if err == nil || string(got) != string(body) {
				t.Fatalf("invalid client instructions overwritten or ignored: %v", err)
			}
		})
	}
}

func TestInstructionLayersOnlyAuthenticatedContextSelectsScope(t *testing.T) {
	config := layerFixtureConfig()
	config.Users["1"] = InstructionLayerProfile{Enabled: true, Prefix: "USER_SCOPE"}
	config.APIKeys["2"] = InstructionLayerProfile{Enabled: true, Prefix: "KEY_SCOPE"}
	body := []byte(`{"instructions":"CLIENT","input":"ordinary task","user":"1","api_key_id":2,"metadata":{"user_id":1,"api_key_id":2},"headers":{"X-User-ID":"1","X-API-Key-ID":"2"}}`)
	for _, test := range []struct {
		name string
		ctx  context.Context
		want string
	}{{"no-context", context.Background(), "PREFIX_FIXTURE"}, {"user-context", context.WithValue(context.Background(), ctxkey.UserID, int64(1)), "USER_SCOPE"}, {"key-context", context.WithValue(context.Background(), ctxkey.APIKeyID, int64(2)), "KEY_SCOPE"}} {
		t.Run(test.name, func(t *testing.T) {
			settings, _ := layerFixtureService(t, config, false)
			got, err := settings.ApplyGroupDefaultSkills(test.ctx, nil, "responses", "fixture", body)
			if err != nil {
				t.Fatal(err)
			}
			text := layerRoot(t, got)["instructions"].(string)
			if !strings.Contains(text, test.want) {
				t.Fatalf("wrong authenticated scope: %q", text)
			}
			if test.name == "no-context" && (strings.Contains(text, "USER_SCOPE") || strings.Contains(text, "KEY_SCOPE")) {
				t.Fatal("body spoof selected a scope")
			}
		})
	}
}

func TestInstructionLayersMonitorProbeReadsNoSettings(t *testing.T) {
	repo := &instructionLayersSettingRepo{values: map[string]string{SettingKeyInstructionLayers: "invalid", SettingKeyGlobalModelSkillRules: "invalid", SettingKeyModelSkills: "invalid"}}
	settings := &SettingService{settingRepo: repo}
	ctx := WithChannelMonitorProbe(context.WithValue(context.WithValue(context.Background(), ctxkey.UserID, int64(1)), ctxkey.APIKeyID, int64(2)))
	for _, protocol := range []string{"responses", "anthropic", "chat", "gemini"} {
		body := []byte("invalid-client-json")
		got, err := settings.ApplyGroupDefaultSkills(ctx, nil, protocol, "fixture", body)
		if err != nil || string(got) != string(body) {
			t.Fatal("monitor request changed")
		}
	}
	if len(repo.reads) != 0 || repo.writes != 0 {
		t.Fatal("monitor accessed settings")
	}
}
