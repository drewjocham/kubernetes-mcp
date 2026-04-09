package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kube-watcher/watcher/internal/config"
)

func TestReferencedEvtFields_Table(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want []string
	}{
		{
			name: "dot and bracket references",
			expr: `evt.restart_count > 3 && evt["waiting_reason"] == "CrashLoopBackOff"`,
			want: []string{"restart_count", "waiting_reason"},
		},
		{
			name: "deduplicates repeated references",
			expr: `evt.restart_count > 1 && evt.restart_count < 10`,
			want: []string{"restart_count"},
		},
		{
			name: "no evt references",
			expr: `1 + 1 == 2`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := referencedEvtFields(tt.expr)
			if len(got) != len(tt.want) {
				t.Fatalf("unexpected field count: got=%v want=%v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("unexpected field[%d]: got=%q want=%q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCELValidationIntegration_LoadConfigAndValidateKinds(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "engine-config.yaml")
	yaml := `
resource_tracking:
  enabled: true
  fields:
    - restart_count
rules:
  - name: PodRule
    kind: Pod
    condition: "evt.restart_count > 2"
    actions: [noop]
  - name: NodeRule
    kind: Node
    condition: "evt.node_memory_pressure == true"
    actions: [noop]
actions:
  noop:
    type: log
settings:
  cel:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	app := &engineApp{cfg: cfg, logger: slog.Default()}
	env, err := app.initCEL(cfg)
	if err != nil {
		t.Fatalf("initCEL: %v", err)
	}
	if err := app.validateCELRules(cfg, env); err != nil {
		t.Fatalf("validateCELRules: %v", err)
	}
}

func TestValidateCELRules_Table(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		expr        string
		wantErrPart string
	}{
		{
			name: "valid boolean expression",
			kind: "Pod",
			expr: `evt.restart_count > 3`,
		},
		{
			name:        "invalid unknown field",
			kind:        "Pod",
			expr:        `evt.non_existent_field > 0`,
			wantErrPart: `unknown evt field`,
		},
		{
			name:        "invalid field for kind",
			kind:        "Node",
			expr:        `evt.restart_count > 0`,
			wantErrPart: `unknown evt field`,
		},
		{
			name: "valid node field for node kind",
			kind: "Node",
			expr: `evt.node_memory_pressure == true`,
		},
		{
			name:        "invalid non boolean expression",
			kind:        "Pod",
			expr:        `1 + 1`,
			wantErrPart: `must return bool`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.WatchConfig{
				Settings: config.Settings{
					CEL: config.CELSettings{Enabled: true},
				},
				Rules: []config.Rule{
					{
						Name:       "test-rule",
						Kind:       tt.kind,
						Expression: tt.expr,
						Actions:    []string{"noop"},
					},
				},
				Actions: map[string]config.Action{
					"noop": {Type: "log"},
				},
			}

			app := &engineApp{
				cfg:    cfg,
				logger: slog.Default(),
			}

			env, err := app.initCEL(cfg)
			if err != nil {
				t.Fatalf("initCEL error: %v", err)
			}

			err = app.validateCELRules(cfg, env)
			if tt.wantErrPart == "" && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.wantErrPart != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrPart)
				}
				if !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErrPart, err)
				}
			}
		})
	}
}
