//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type instructionLayerSettingRepo struct {
	SettingRepository
	values map[string]string
	reads  []string
}

func (r *instructionLayerSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.reads = append(r.reads, key)
	return r.values[key], nil
}

func instructionLayerTestProfile(prefix string) InstructionLayerProfile {
	return InstructionLayerProfile{
		Enabled: true, Prefix: prefix, Suffix: prefix + "-suffix",
		ModelSuffixes: map[string]string{},
	}
}

func instructionLayerTestContext(groupID, userID, apiKeyID int64) context.Context {
	ctx := context.Background()
	if groupID > 0 {
		ctx = context.WithValue(ctx, ctxkey.AuthenticatedGroupID, groupID)
	}
	if userID > 0 {
		ctx = context.WithValue(ctx, ctxkey.UserID, userID)
	}
	if apiKeyID > 0 {
		ctx = context.WithValue(ctx, ctxkey.APIKeyID, apiKeyID)
	}
	return ctx
}

func instructionLayerTestJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func TestInstructionLayersLegacyConfigAndGroupValidation(t *testing.T) {
	legacy := `{"enabled":true,"default":{"enabled":true,"prefix":"  old\n","suffix":"\told tail ","model_suffixes":{"exact":""}},"users":{},"api_keys":{}}`
	repo := &instructionLayerSettingRepo{values: map[string]string{SettingKeyInstructionLayers: legacy}}
	service := &SettingService{settingRepo: repo}
	config, err := service.GetInstructionLayers(context.Background())
	require.NoError(t, err)
	require.NotNil(t, config.Groups)
	require.Empty(t, config.Groups)
	require.Equal(t, "  old\n", config.Default.Prefix)
	require.Equal(t, "\told tail ", config.Default.Suffix)
	require.Contains(t, config.Default.ModelSuffixes, "exact")
	require.Empty(t, config.Default.ModelSuffixes["exact"])

	for _, id := range []string{"0", "01", "+1", "-1", " 1", "1.0", "9223372036854775808"} {
		t.Run("invalid-group-"+id, func(t *testing.T) {
			candidate := emptyInstructionLayersConfig()
			candidate.Groups[id] = instructionLayerTestProfile("group")
			_, err := normalizeInstructionLayers(candidate)
			require.Error(t, err)
		})
	}
	config.Groups["9223372036854775807"] = instructionLayerTestProfile("large-id")
	_, err = normalizeInstructionLayers(config)
	require.NoError(t, err)
	config = emptyInstructionLayersConfig()
	for id := 1; id <= 128; id++ {
		config.Groups[strconv.Itoa(id)] = instructionLayerTestProfile("group")
	}
	_, err = normalizeInstructionLayers(config)
	require.ErrorContains(t, err, "128")
	delete(config.Groups, "128")
	_, err = normalizeInstructionLayers(config)
	require.NoError(t, err)
}

func TestInstructionLayersGroupScopePrecedenceAndOptOut(t *testing.T) {
	config := emptyInstructionLayersConfig()
	config.Enabled = true
	config.Default = instructionLayerTestProfile("default")
	config.Groups["42"] = instructionLayerTestProfile("group")
	config.Users["7"] = instructionLayerTestProfile("user")
	config.APIKeys["100"] = instructionLayerTestProfile("key")
	for _, test := range []struct {
		name                   string
		groupID, userID, keyID int64
		prefix                 string
	}{
		{"default", 0, 0, 0, "default"},
		{"group", 42, 0, 0, "group"},
		{"user", 42, 7, 0, "user"},
		{"key", 42, 7, 100, "key"},
		{"unconfigured-group", 43, 0, 0, "default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, suffix := resolveInstructionLayerText(
				instructionLayerTestContext(test.groupID, test.userID, test.keyID), config, "test-model")
			require.Equal(t, test.prefix, prefix)
			require.Equal(t, test.prefix+"-suffix", suffix)
		})
	}

	group := config.Groups["42"]
	group.Enabled = false
	config.Groups["42"] = group
	for _, enabled := range []bool{true, false} {
		config.Enabled = enabled
		ctx := instructionLayerTestContext(42, 7, 100)
		require.True(t, instructionLayerGroupDisabled(ctx, config))
		prefix, suffix := resolveInstructionLayerText(ctx, config, "test-model")
		require.Empty(t, prefix)
		require.Empty(t, suffix)
	}
	config.Enabled = true
	delete(config.Groups, "42")
	prefix, _ := resolveInstructionLayerText(instructionLayerTestContext(42, 0, 0), config, "test-model")
	require.Equal(t, "default", prefix, "deleting the group override restores inheritance")
	prefix, _ = resolveInstructionLayerText(instructionLayerTestContext(42, 7, 100), config, "test-model")
	require.Equal(t, "key", prefix)
}

func TestInstructionLayersGroupExactBlankModelOverridesWildcard(t *testing.T) {
	config := emptyInstructionLayersConfig()
	config.Enabled = true
	config.Default = instructionLayerTestProfile("default")
	group := instructionLayerTestProfile("  group before\n")
	group.ModelSuffixes = map[string]string{"exact": "", "*": "\twildcard\n"}
	config.Groups["42"] = group
	ctx := instructionLayerTestContext(42, 0, 0)
	prefix, suffix := resolveInstructionLayerText(ctx, config, "exact")
	require.Equal(t, group.Prefix, prefix)
	require.Empty(t, suffix)
	_, suffix = resolveInstructionLayerText(ctx, config, "other")
	require.Equal(t, "\twildcard\n", suffix)
	delete(group.ModelSuffixes, "*")
	config.Groups["42"] = group
	_, suffix = resolveInstructionLayerText(ctx, config, "other")
	require.Equal(t, group.Suffix, suffix)
}

func TestInstructionLayersDisabledGroupSkipsAllProtocolsAndSkills(t *testing.T) {
	bodies := map[string]string{
		"responses":        "{\n \"model\":\"test-model\", \"instructions\":\"client only\", \"input\":\"hi\", \"metadata\":{\"group_id\":99}\n}",
		"anthropic":        `{"model":"test-model","system":[{"type":"text","text":"client only","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hi"}]}`,
		"chat_completions": `{"model":"test-model","messages":[{"role":"developer","content":"client only"},{"role":"user","content":"hi"}]}`,
		"gemini":           `{"systemInstruction":{"parts":[{"text":"client only"}]},"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
	}
	for protocol, body := range bodies {
		for _, enabled := range []bool{true, false} {
			t.Run(protocol+"/"+strconv.FormatBool(enabled), func(t *testing.T) {
				config := emptyInstructionLayersConfig()
				config.Enabled = enabled
				config.Default = instructionLayerTestProfile("default")
				config.Groups["42"] = InstructionLayerProfile{Enabled: false}
				config.Groups["99"] = instructionLayerTestProfile("fallback")
				config.Users["7"] = instructionLayerTestProfile("user")
				config.APIKeys["100"] = instructionLayerTestProfile("key")
				repo := &instructionLayerSettingRepo{values: map[string]string{
					SettingKeyInstructionLayers:     instructionLayerTestJSON(t, config),
					SettingKeyGlobalModelSkillRules: "invalid JSON must not be read for opted-out groups",
					SettingKeyModelSkills:           "invalid JSON must not be read for opted-out groups",
				}}
				settings := &SettingService{settingRepo: repo}
				ctx := instructionLayerTestContext(42, 7, 100)
				ctx = context.WithValue(ctx, ctxkey.Group, &Group{ID: 99})
				group := &Group{ID: 99, DefaultSkillIDs: []string{"group-skill"}}
				actual, err := settings.ApplyGroupDefaultSkills(ctx, group, protocol, "test-model", []byte(body))
				require.NoError(t, err)
				require.Equal(t, []byte(body), actual, "opt-out must preserve every original request byte")
				require.Equal(t, []string{SettingKeyInstructionLayers}, repo.reads)
			})
		}
	}
}

func TestInstructionLayersEnabledGroupAndLegacySwitchPreserveSkills(t *testing.T) {
	for _, test := range []struct {
		name                                         string
		master, groupEnabled, keyDisabled, bindGroup bool
		expectedPrefix                               string
	}{
		{"group", true, true, false, true, "group-before"},
		{"master-off-keeps-skills", false, true, false, true, ""},
		{"disabled-key-keeps-skills", true, true, true, true, ""},
		{"body-and-active-group-cannot-spoof-scope", true, false, false, false, "default-before"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := emptyInstructionLayersConfig()
			config.Enabled = test.master
			config.Default = instructionLayerTestProfile("default-before")
			config.Groups["42"] = instructionLayerTestProfile("group-before")
			group := config.Groups["42"]
			group.Enabled = test.groupEnabled
			config.Groups["42"] = group
			if test.keyDisabled {
				config.APIKeys["100"] = InstructionLayerProfile{Enabled: false}
			}
			repo := &instructionLayerSettingRepo{values: map[string]string{
				SettingKeyInstructionLayers:     instructionLayerTestJSON(t, config),
				SettingKeyGlobalModelSkillRules: instructionLayerTestJSON(t, ModelSkillRuleConfig{DefaultSkillIDs: []string{"skill"}}),
				SettingKeyModelSkills:           instructionLayerTestJSON(t, []ModelSkill{{ID: "skill", Name: "Skill", Content: "automatic-skill-body"}}),
			}}
			settings := &SettingService{settingRepo: repo}
			groupID := int64(0)
			if test.bindGroup {
				groupID = 42
			}
			ctx := instructionLayerTestContext(groupID, 7, 100)
			ctx = context.WithValue(ctx, ctxkey.Group, &Group{ID: 42})
			body := []byte(`{"model":"test-model","instructions":"client-original","input":"hello","group_id":42,"metadata":{"group_id":42}}`)
			actual, err := settings.ApplyGroupDefaultSkills(ctx, &Group{ID: 42}, "responses", "test-model", body)
			require.NoError(t, err)
			require.Contains(t, string(actual), "client-original")
			require.Contains(t, string(actual), "automatic-skill-body")
			if test.expectedPrefix != "" {
				require.Contains(t, string(actual), test.expectedPrefix)
			} else {
				require.NotContains(t, string(actual), "group-before")
				require.NotContains(t, string(actual), "default-before")
			}
			again, err := settings.ApplyGroupDefaultSkills(ctx, &Group{ID: 42}, "responses", "test-model", actual)
			require.NoError(t, err)
			require.Equal(t, actual, again)
		})
	}
}

func TestInstructionLayersGroupTextLimitsAndUnrecognizedContext(t *testing.T) {
	config := emptyInstructionLayersConfig()
	config.Enabled = true
	config.Default = instructionLayerTestProfile("default")
	group := instructionLayerTestProfile("group")
	group.Prefix = strings.Repeat("x", maxInstructionLayerTextBytes+1)
	config.Groups["42"] = group
	_, err := normalizeInstructionLayers(config)
	require.ErrorContains(t, err, "groups[42].prefix")
	config.Groups["42"] = InstructionLayerProfile{Enabled: false}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedGroupID, "42")
	require.False(t, instructionLayerGroupDisabled(ctx, config), "only authenticated int64 IDs are accepted")
	prefix, _ := resolveInstructionLayerText(ctx, config, "test-model")
	require.Equal(t, "default", prefix)
}
