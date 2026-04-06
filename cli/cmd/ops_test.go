package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComponentCommand_Table(t *testing.T) {
	cfg := opsConfig{pollInterval: "45s"}
	tests := []struct {
		name      string
		component string
		wantPart  string
	}{
		{name: "mcp includes interval", component: "mcp", wantPart: "--interval"},
		{name: "watcher uses config", component: "watcher", wantPart: "--config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := componentCommand(tt.component, cfg)
			got := strings.Join(cmd, " ")
			if !strings.Contains(got, tt.wantPart) {
				t.Fatalf("expected %q in %q", tt.wantPart, got)
			}
		})
	}
}

func TestContainerName_Table(t *testing.T) {
	tests := []struct {
		name      string
		cfg       opsConfig
		component string
		want      string
	}{
		{name: "default prefix", cfg: opsConfig{namePrefix: "kw"}, component: "mcp", want: "kw-mcp"},
		{name: "trim prefix", cfg: opsConfig{namePrefix: "  kw  "}, component: "watcher", want: "kw-watcher"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containerName(tt.component, tt.cfg); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestKubeManifest_WatcherConfigMap(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "watcher.yaml")
	if err := os.WriteFile(cfgPath, []byte("rules: []\n"), 0o600); err != nil {
		t.Fatalf("write watcher config: %v", err)
	}

	cfg := opsConfig{
		image:         "example/image:latest",
		namePrefix:    "kw",
		kubeNamespace: "kubewatcher",
		kubeStorage:   "1Gi",
		watcherConfig: cfgPath,
	}

	manifest, err := kubeManifest("watcher", cfg)
	if err != nil {
		t.Fatalf("kubeManifest error: %v", err)
	}

	assertContains := func(substr string) {
		t.Helper()
		if !strings.Contains(manifest, substr) {
			t.Fatalf("missing %q in manifest:\n%s", substr, manifest)
		}
	}

	assertContains("kind: ConfigMap")
	assertContains("name: kw-watcher-config")
	assertContains("kind: PersistentVolumeClaim")
	assertContains("storage: 1Gi")
	assertContains("kind: Deployment")
	assertContains("watcher-config")
}
