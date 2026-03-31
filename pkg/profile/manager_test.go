package profile

import (
	"errors"
	"path/filepath"
	"testing"
)

type fakeResolver struct {
	values map[string]string
	err    error
}

func (f fakeResolver) Resolve(provider, ref string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.values[provider+":"+ref], nil
}

func TestManager_CreateAndSetActive(t *testing.T) {
	tests := []struct {
		name        string
		createName  string
		description string
		wantErr     bool
	}{
		{
			name:        "Success - Create profile",
			createName:  "dev",
			description: "development profile",
			wantErr:     false,
		},
		{
			name:        "Failure - Empty name",
			createName:  "",
			description: "invalid profile",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			manager := NewManager(filepath.Join(tmpDir, "profiles.yaml"))

			err := manager.Create(tt.createName, tt.description)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("create returned unexpected error: %v", err)
			}

			cfg, err := manager.Load()
			if err != nil {
				t.Fatalf("load returned unexpected error: %v", err)
			}
			if cfg.Active != tt.createName {
				t.Fatalf("expected active profile %q, got %q", tt.createName, cfg.Active)
			}
		})
	}
}

func TestManager_SetConfigOverrideAndActiveOverrides(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		overrideKey string
		overrideVal string
		expectedVal string
		wantErr     bool
	}{
		{
			name:        "Success - Set active override",
			profileName: "dev",
			overrideKey: "ops.target",
			overrideVal: "docker",
			expectedVal: "docker",
			wantErr:     false,
		},
		{
			name:        "Failure - Profile missing",
			profileName: "missing",
			overrideKey: "ops.target",
			overrideVal: "docker",
			expectedVal: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			manager := NewManager(filepath.Join(tmpDir, "profiles.yaml"))
			if err := manager.Create("dev", "development profile"); err != nil {
				t.Fatalf("bootstrap create returned error: %v", err)
			}

			err := manager.SetConfigOverride(tt.profileName, tt.overrideKey, tt.overrideVal)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("set override returned unexpected error: %v", err)
			}

			overrides, err := manager.ActiveConfigOverrides()
			if err != nil {
				t.Fatalf("active overrides returned error: %v", err)
			}
			if overrides[tt.overrideKey] != tt.expectedVal {
				t.Fatalf("expected override value %q, got %q", tt.expectedVal, overrides[tt.overrideKey])
			}
		})
	}
}

func TestManager_ResolveEnv(t *testing.T) {
	tests := []struct {
		name           string
		setupSecretRef bool
		resolverErr    error
		wantErr        bool
		expectedValue  string
	}{
		{
			name:           "Success - Resolve env and secret refs",
			setupSecretRef: true,
			resolverErr:    nil,
			wantErr:        false,
			expectedValue:  "token-value",
		},
		{
			name:           "Failure - Resolver returns error",
			setupSecretRef: true,
			resolverErr:    errors.New("resolver failed"),
			wantErr:        true,
			expectedValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			manager := NewManager(filepath.Join(tmpDir, "profiles.yaml")).WithResolver(fakeResolver{
				values: map[string]string{"env:MCP_API_TOKEN": "token-value"},
				err:    tt.resolverErr,
			})
			if err := manager.Create("dev", "development profile"); err != nil {
				t.Fatalf("bootstrap create returned error: %v", err)
			}
			if err := manager.SetEnvValue("dev", "KUBECONFIG", "/tmp/kubeconfig"); err != nil {
				t.Fatalf("set env returned error: %v", err)
			}
			if tt.setupSecretRef {
				if err := manager.SetSecretRef("dev", "MCP_API_TOKEN", "env", "MCP_API_TOKEN"); err != nil {
					t.Fatalf("set secret ref returned error: %v", err)
				}
			}

			resolved, err := manager.ResolveEnv("dev")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve env returned error: %v", err)
			}
			if resolved["MCP_API_TOKEN"] != tt.expectedValue {
				t.Fatalf("expected MCP_API_TOKEN %q, got %q", tt.expectedValue, resolved["MCP_API_TOKEN"])
			}
		})
	}
}

func TestMaskedEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		checkKey string
		expected string
		wantErr  bool
	}{
		{
			name: "Success - Mask token values",
			input: map[string]string{
				"MCP_TOKEN":  "abc123",
				"KUBECONFIG": "/tmp/kubeconfig",
			},
			checkKey: "MCP_TOKEN",
			expected: "******",
			wantErr:  false,
		},
		{
			name: "Success - Keep non-secret values",
			input: map[string]string{
				"MCP_TOKEN":  "abc123",
				"KUBECONFIG": "/tmp/kubeconfig",
			},
			checkKey: "KUBECONFIG",
			expected: "/tmp/kubeconfig",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := MaskedEnv(tt.input)
			if masked[tt.checkKey] != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, masked[tt.checkKey])
			}
		})
	}
}
