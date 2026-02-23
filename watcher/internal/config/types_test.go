package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	writeYAML := func(name string, data any) string {
		path := filepath.Join(dir, name)
		bytes, err := yaml.Marshal(data)
		require.NoError(t, err)
		err = os.WriteFile(path, bytes, 0644)
		require.NoError(t, err)
		return path
	}

	validPath := writeYAML("valid.yaml", &WatchConfig{
		Rules: []Rule{{
			Name:       "test-rule",
			Kind:       "Pod",
			Conditions: []Condition{{Field: "status.phase", Operator: "eq", Value: "Failed"}},
			Actions:    []string{"log"},
		}},
		Actions: map[string]Action{"log": {Type: "log"}},
	})

	merge1Path := writeYAML("merge1.yaml", &WatchConfig{
		ResourceTracking: ResourceTrackingConfig{Enabled: true, Storage: "memory"},
		Rules:            []Rule{{Name: "rule1", Actions: []string{"a1"}, Conditions: []Condition{{Field: "f1"}}}},
		Actions:          map[string]Action{"a1": {Type: "log"}},
	})

	merge2Path := writeYAML("merge2.yaml", &WatchConfig{
		ResourceTracking: ResourceTrackingConfig{Storage: "disk", Path: "/tmp/db"},
		Rules:            []Rule{{Name: "rule2", Actions: []string{"a2"}, Conditions: []Condition{{Field: "f2"}}}},
		Actions:          map[string]Action{"a2": {Type: "slack"}},
	})

	invalidPath := filepath.Join(dir, "invalid.yaml")
	_ = os.WriteFile(invalidPath, []byte("not yaml"), 0644)

	emptyPath := filepath.Join(dir, "empty.yaml")
	_ = os.WriteFile(emptyPath, []byte(""), 0644)

	t.Run("DefaultPathsMerging", func(t *testing.T) {
		old := DefaultConfigPaths
		defer func() { DefaultConfigPaths = old }()
		DefaultConfigPaths = func() []string { return []string{merge1Path, merge2Path} }

		cfg, err := Load("")
		require.NoError(t, err)
		assert.True(t, cfg.ResourceTracking.Enabled)
		assert.Equal(t, "disk", cfg.ResourceTracking.Storage)
		assert.Len(t, cfg.Rules, 2)
		assert.Len(t, cfg.Actions, 2)
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
		check   func(*testing.T, *WatchConfig)
	}{
		{
			name: "ValidConfig",
			path: validPath,
			check: func(t *testing.T, cfg *WatchConfig) {
				assert.Equal(t, "test-rule", cfg.Rules[0].Name)
			},
		},
		{
			name:    "InvalidYAML",
			path:    invalidPath,
			wantErr: true,
		},
		{
			name:    "FileNotFound",
			path:    "missing.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestWatchConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *WatchConfig
		wantErr error
	}{
		{
			name: "Valid",
			cfg: &WatchConfig{
				Rules:   []Rule{{Name: "r", Conditions: []Condition{{Field: "f"}}, Actions: []string{"a"}}},
				Actions: map[string]Action{"a": {Type: "log"}},
			},
		},
		{
			name:    "NoRules",
			cfg:     &WatchConfig{},
			wantErr: ErrNoRules,
		},
		{
			name:    "MissingName",
			cfg:     &WatchConfig{Rules: []Rule{{Actions: []string{"a"}}}},
			wantErr: ErrRuleMissingName,
		},
		{
			name: "UnknownAction",
			cfg: &WatchConfig{
				Rules: []Rule{{Name: "r", Conditions: []Condition{{Field: "f"}}, Actions: []string{"ghost"}}},
			},
			wantErr: assert.AnError, // Generic check for unknown action
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWatchConfig_Merge(t *testing.T) {
	base := &WatchConfig{
		ResourceTracking: ResourceTrackingConfig{Enabled: true, Storage: "memory", Fields: []string{"cpu"}},
		Rules:            []Rule{{Name: "rule1"}},
		Actions:          map[string]Action{"a1": {Type: "log"}},
	}
	other := WatchConfig{
		ResourceTracking: ResourceTrackingConfig{Storage: "disk", Fields: []string{"mem"}},
		Rules:            []Rule{{Name: "rule2"}},
		Actions:          map[string]Action{"a2": {Type: "slack"}},
		Settings:         Settings{QueueDepth: 500},
	}

	base.merge(other)

	assert.True(t, base.ResourceTracking.Enabled)
	assert.Equal(t, "disk", base.ResourceTracking.Storage)
	assert.ElementsMatch(t, []string{"cpu", "mem"}, base.ResourceTracking.Fields)
	assert.Len(t, base.Rules, 2)
	assert.Contains(t, base.Actions, "a2")
	assert.Equal(t, 500, base.Settings.QueueDepth)
}

func Test_MergeStrings(t *testing.T) {
	tests := []struct {
		name  string
		base  []string
		extra []string
		want  []string
	}{
		{"Empty", []string{}, []string{}, []string{}},
		{"Overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"Cleaning", []string{" a "}, []string{"", "b ", "a"}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeStrings(tt.base, tt.extra)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
