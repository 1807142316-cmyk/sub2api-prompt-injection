//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type customizationBenchmarkSettingRepo struct {
	SettingRepository
	mu         sync.RWMutex
	values     map[string]string
	layerReads atomic.Uint64
	ruleReads  atomic.Uint64
	skillReads atomic.Uint64
}

func (r *customizationBenchmarkSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	switch key {
	case SettingKeyInstructionLayers:
		r.layerReads.Add(1)
	case SettingKeyGlobalModelSkillRules:
		r.ruleReads.Add(1)
	case SettingKeyModelSkills:
		r.skillReads.Add(1)
	}
	r.mu.RLock()
	value, ok := r.values[key]
	r.mu.RUnlock()
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *customizationBenchmarkSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	r.values[key] = value
	r.mu.Unlock()
	return nil
}

func (r *customizationBenchmarkSettingRepo) resetReads() {
	r.layerReads.Store(0)
	r.ruleReads.Store(0)
	r.skillReads.Store(0)
}

func customizationBenchmarkJSON(tb testing.TB, value any) []byte {
	tb.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func customizationBenchmarkFixture(tb testing.TB, protocol string, inputBytes int) (context.Context, *SettingService, *customizationBenchmarkSettingRepo, *Group, []byte) {
	tb.Helper()
	layers := InstructionLayersConfig{
		Enabled: true,
		Default: InstructionLayerProfile{
			Enabled: true, Prefix: "PREFIX_FIXTURE", Suffix: "SUFFIX_FIXTURE",
			ModelSuffixes: map[string]string{"bench-model": "MODEL_SUFFIX_FIXTURE"},
		},
	}
	skills := []ModelSkill{
		{ID: "base", Name: "base", Content: "BASE_SKILL_FIXTURE\n" + strings.Repeat("base rule\n", 256)},
		{ID: "task", Name: "task", Content: `<!-- sub2api:skill-route:v1 {"id":"inspect","any_regex":["inspect"],"not_regex":["skip"]} -->` +
			"\nTASK_SKILL_FIXTURE\n" + strings.Repeat("task rule\n", 256)},
		{ID: "group", Name: "group", Content: "GROUP_SKILL_FIXTURE\n" + strings.Repeat("group rule\n", 256)},
	}
	repo := &customizationBenchmarkSettingRepo{values: map[string]string{
		SettingKeyInstructionLayers:     string(customizationBenchmarkJSON(tb, layers)),
		SettingKeyModelSkills:           string(customizationBenchmarkJSON(tb, skills)),
		SettingKeyGlobalModelSkillRules: string(customizationBenchmarkJSON(tb, ModelSkillRuleConfig{DefaultSkillIDs: []string{"base", "task"}})),
	}}
	task := strings.Repeat("x", inputBytes) + " inspect"
	root := map[string]any{"model": "bench-model", "metadata": map[string]string{"fixture": "preserve"}}
	switch protocol {
	case "responses":
		root["instructions"] = "CLIENT_FIXTURE"
		root["input"] = task
	case "anthropic":
		root["system"] = "CLIENT_FIXTURE"
		root["messages"] = []any{map[string]any{"role": "user", "content": task}}
	case "chat_completions":
		root["messages"] = []any{
			map[string]any{"role": "developer", "content": "CLIENT_FIXTURE"},
			map[string]any{"role": "user", "content": task},
		}
	case "gemini":
		root["systemInstruction"] = map[string]any{"parts": []any{map[string]any{"text": "CLIENT_FIXTURE"}}}
		root["contents"] = []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": task}}}}
	default:
		tb.Fatalf("unknown fixture protocol %q", protocol)
	}
	ctx := context.WithValue(context.Background(), ctxkey.AuthenticatedGroupID, int64(42))
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(101))
	ctx = context.WithValue(ctx, ctxkey.APIKeyID, int64(202))
	group := &Group{ID: 99, DefaultSkillIDs: []string{"task", "group"}}
	return ctx, &SettingService{settingRepo: repo}, repo, group, customizationBenchmarkJSON(tb, root)
}

func TestCustomizationWarmSkillsRespectsAuthenticatedGroupOptOut(t *testing.T) {
	for _, protocol := range []string{"responses", "anthropic", "chat_completions", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			ctx, settings, repo, routedGroup, body := customizationBenchmarkFixture(t, protocol, 1024)
			warm, err := settings.ApplyGroupDefaultSkills(ctx, routedGroup, protocol, "bench-model", body)
			require.NoError(t, err)
			require.Contains(t, string(warm), "TASK_SKILL_FIXTURE")

			config, err := settings.GetInstructionLayers(ctx)
			require.NoError(t, err)
			config.Enabled = false
			config.Groups["42"] = InstructionLayerProfile{Enabled: false}
			config.Groups["99"] = InstructionLayerProfile{Enabled: true, Prefix: "ROUTED_GROUP"}
			config.Users["101"] = InstructionLayerProfile{Enabled: true, Prefix: "USER_OVERRIDE"}
			config.APIKeys["202"] = InstructionLayerProfile{Enabled: true, Prefix: "KEY_OVERRIDE"}
			_, err = settings.UpdateInstructionLayers(ctx, config)
			require.NoError(t, err)
			repo.resetReads()

			actual, err := settings.ApplyGroupDefaultSkills(ctx, routedGroup, protocol, "bench-model", body)
			require.NoError(t, err)
			require.Equal(t, body, actual)
			require.Equal(t, uint64(1), repo.layerReads.Load())
			require.Zero(t, repo.ruleReads.Load(), "group opt-out must run before Skill rule access")
			require.Zero(t, repo.skillReads.Load(), "warm Skills must not bypass the authenticated group opt-out")
		})
	}
}

func TestCustomizationSettingsWritesRefreshTaskRouting(t *testing.T) {
	ctx, settings, repo, _, body := customizationBenchmarkFixture(t, "responses", 1024)
	first, err := settings.ApplyGroupDefaultSkills(ctx, nil, "responses", "bench-model", body)
	require.NoError(t, err)
	require.Contains(t, string(first), "TASK_SKILL_FIXTURE")

	skills, err := settings.GetModelSkills(ctx)
	require.NoError(t, err)
	for i := range skills {
		if skills[i].ID == "task" {
			skills[i].Content = `<!-- sub2api:skill-route:v1 {"id":"deploy","any_regex":["deploy"],"not_regex":[]} -->` + "\nUPDATED_TASK_FIXTURE"
		}
	}
	require.NoError(t, settings.saveModelSkills(ctx, skills))
	second, err := settings.ApplyGroupDefaultSkills(ctx, nil, "responses", "bench-model", body)
	require.NoError(t, err)
	require.NotContains(t, string(second), "TASK_SKILL_FIXTURE")
	require.NotContains(t, string(second), "UPDATED_TASK_FIXTURE")
	require.Contains(t, string(second), "BASE_SKILL_FIXTURE")
	require.NotContains(t, string(second), "sub2api:skill-route:")

	_, err = settings.UpdateGlobalModelSkillRules(ctx, ModelSkillRuleConfig{DefaultSkillIDs: []string{"group"}})
	require.NoError(t, err)
	third, err := settings.ApplyGroupDefaultSkills(ctx, nil, "responses", "bench-model", body)
	require.NoError(t, err)
	require.Contains(t, string(third), "GROUP_SKILL_FIXTURE")
	require.NotContains(t, string(third), "BASE_SKILL_FIXTURE")
	require.Contains(t, string(third), "CLIENT_FIXTURE")

	repo.resetReads()
	replay, err := settings.ApplyGroupDefaultSkills(ctx, nil, "responses", "bench-model", third)
	require.NoError(t, err)
	require.Equal(t, third, replay)
	require.Zero(t, repo.layerReads.Load()+repo.ruleReads.Load()+repo.skillReads.Load())
}

func BenchmarkCustomizationSkills(b *testing.B) {
	for _, protocol := range []string{"responses", "anthropic", "chat_completions", "gemini"} {
		for _, inputBytes := range []int{1024, 256 * 1024} {
			b.Run(fmt.Sprintf("%s/warm/%dKiB", protocol, inputBytes/1024), func(b *testing.B) {
				benchmarkCustomizationSkills(b, protocol, inputBytes, "warm")
			})
		}
	}
	for _, mode := range []string{"cold", "group_opt_out", "monitor_probe", "already_injected"} {
		b.Run("responses/"+mode+"/1KiB", func(b *testing.B) {
			benchmarkCustomizationSkills(b, "responses", 1024, mode)
		})
	}
}

func benchmarkCustomizationSkills(b *testing.B, protocol string, inputBytes int, mode string) {
	ctx, settings, repo, group, body := customizationBenchmarkFixture(b, protocol, inputBytes)
	warm, err := settings.ApplyGroupDefaultSkills(ctx, group, protocol, "bench-model", body)
	if err != nil || !bytes.Contains(warm, []byte("TASK_SKILL_FIXTURE")) {
		b.Fatalf("invalid warm fixture: %v", err)
	}
	switch mode {
	case "group_opt_out":
		config, err := settings.GetInstructionLayers(ctx)
		if err != nil {
			b.Fatal(err)
		}
		config.Groups["42"] = InstructionLayerProfile{Enabled: false}
		if _, err := settings.UpdateInstructionLayers(ctx, config); err != nil {
			b.Fatal(err)
		}
	case "monitor_probe":
		ctx = WithChannelMonitorProbe(ctx)
	case "already_injected":
		body = warm
	}
	repo.resetReads()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		active := settings
		if mode == "cold" {
			active = &SettingService{settingRepo: repo}
		}
		if _, err := active.ApplyGroupDefaultSkills(ctx, group, protocol, "bench-model", body); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(repo.layerReads.Load())/float64(b.N), "layer_reads/op")
	b.ReportMetric(float64(repo.ruleReads.Load())/float64(b.N), "rule_reads/op")
	b.ReportMetric(float64(repo.skillReads.Load())/float64(b.N), "skill_reads/op")
}
