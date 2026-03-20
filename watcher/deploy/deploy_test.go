package deploy

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeAndValidate_Table(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantErrPart string
	}{
		{
			name: "valid deploy derives defaults from cluster",
			cfg: Config{
				Action:             "deploy",
				Target:             "kube",
				ClusterName:        "Dev_Cluster-01",
				PrometheusEndpoint: "http://prometheus:9090/metrics",
				DockerDataDir:      DefaultDataDir,
				DockerConfigPath:   DefaultDockerConfig,
			},
		},
		{
			name: "status action is accepted",
			cfg: Config{
				Action:      "status",
				Target:      "docker",
				ClusterName: "prod",
			},
		},
		{
			name: "logs action is accepted",
			cfg: Config{
				Action:      "logs",
				Target:      "kube",
				ClusterName: "prod",
			},
		},
		{
			name: "missing cluster name",
			cfg: Config{
				Action:             "deploy",
				Target:             "kube",
				PrometheusEndpoint: "http://prometheus:9090/metrics",
			},
			wantErrPart: "--cluster-name is required",
		},
		{
			name: "deploy requires prometheus endpoint",
			cfg: Config{
				Action:      "deploy",
				Target:      "kube",
				ClusterName: "prod",
			},
			wantErrPart: "--prometheus-endpoint is required for deploy",
		},
		{
			name: "cleanup does not require prometheus endpoint",
			cfg: Config{
				Action:      "cleanup",
				Target:      "kube",
				ClusterName: "prod",
			},
		},
		{
			name: "invalid action",
			cfg: Config{
				Action:      "bad",
				Target:      "kube",
				ClusterName: "prod",
			},
			wantErrPart: "unsupported action",
		},
		{
			name: "invalid target",
			cfg: Config{
				Action:      "deploy",
				Target:      "bad",
				ClusterName: "prod",
			},
			wantErrPart: "unsupported target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAndValidate(tt.cfg)
			if tt.wantErrPart != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrPart)
				}
				if !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrPart, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.cfg.ClusterName != "" {
				if got.Name == "" {
					t.Fatalf("expected derived deployment name, got empty")
				}
				if !strings.Contains(got.Name, "kube-anomaly-detection") {
					t.Fatalf("expected derived name to include base app name, got %q", got.Name)
				}
				if got.DockerDataDir == DefaultDataDir {
					t.Fatalf("expected dockerDataDir to be cluster-specific, got default %q", got.DockerDataDir)
				}
				if got.DockerConfigPath == DefaultDockerConfig {
					t.Fatalf("expected dockerConfigPath to be cluster-specific, got default %q", got.DockerConfigPath)
				}
				if got.PVCName == "" {
					t.Fatalf("expected default pvc name to be set")
				}
				if got.PVCSize == "" {
					t.Fatalf("expected default pvc size to be set")
				}
				if got.TailLines <= 0 {
					t.Fatalf("expected positive tail lines, got %d", got.TailLines)
				}
			}
		})
	}
}

func TestSanitizeForResourceName_Table(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Prod_Cluster-01", want: "prod-cluster-01"},
		{in: "  $$$  ", want: ""},
		{in: "a--b__c", want: "a-b-c"},
		{in: strings.Repeat("a", 80), want: strings.Repeat("a", 63)},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := SanitizeForResourceName(tt.in)
			if got != tt.want {
				t.Fatalf("SanitizeForResourceName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestKubeManifest_UsesSecretAndPVC(t *testing.T) {
	cfg := Config{
		Action:             "deploy",
		Target:             "kube",
		ClusterName:        "dev-cluster",
		Name:               "kube-anomaly-detection-dev-cluster",
		Namespace:          "kubewatcher",
		Image:              "kube-anomaly-detection:latest",
		PrometheusEndpoint: "http://prometheus.monitoring.svc:9090/metrics",
		PVCName:            "kube-anomaly-detection-dev-cluster-data",
		PVCSize:            "10Gi",
	}

	manifest := KubeManifest(cfg)

	assertContains := func(substr string) {
		t.Helper()
		if !strings.Contains(manifest, substr) {
			t.Fatalf("manifest missing expected fragment: %q\nmanifest:\n%s", substr, manifest)
		}
	}

	assertContains("kind: Secret")
	assertContains("stringData:")
	assertContains("kind: PersistentVolumeClaim")
	assertContains("claimName: kube-anomaly-detection-dev-cluster-data")
	assertContains("storage: 10Gi")
	assertContains("secretName: kube-anomaly-detection-dev-cluster-config")
	assertContains("name: CLUSTER_NAME")
	assertContains(`value: "dev-cluster"`)
}

func TestNormalizeAndValidate_GoogleChatWebhookEnv(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("GOOGLE_CHAT_WEBHOOK_URL")
	defer func() {
		// Restore original value
		if originalValue != "" {
			_ = os.Setenv("GOOGLE_CHAT_WEBHOOK_URL", originalValue)
		} else {
			_ = os.Unsetenv("GOOGLE_CHAT_WEBHOOK_URL")
		}
	}()

	// Test 1: Environment variable should be read when GoogleChatWebhook is empty
	_ = os.Setenv("GOOGLE_CHAT_WEBHOOK_URL", "https://chat.example.com/webhook")
	cfg := Config{
		Action:             "deploy",
		Target:             "kube",
		ClusterName:        "test-cluster",
		PrometheusEndpoint: "http://prometheus:9090/metrics",
		GoogleChatWebhook:  "", // Empty, should read from env
	}

	result, err := NormalizeAndValidate(cfg)
	if err != nil {
		t.Fatalf("NormalizeAndValidate failed: %v", err)
	}

	if result.GoogleChatWebhook != "https://chat.example.com/webhook" {
		t.Errorf("Expected GoogleChatWebhook to be read from environment variable, got: %q", result.GoogleChatWebhook)
	}

	// Test 2: Command-line flag should take precedence over environment variable
	cfg.GoogleChatWebhook = "https://flag.example.com/webhook"
	result, err = NormalizeAndValidate(cfg)
	if err != nil {
		t.Fatalf("NormalizeAndValidate failed: %v", err)
	}

	if result.GoogleChatWebhook != "https://flag.example.com/webhook" {
		t.Errorf("Expected command-line flag to take precedence, got: %q", result.GoogleChatWebhook)
	}

	// Test 3: Empty environment variable should not override empty flag
	_ = os.Unsetenv("GOOGLE_CHAT_WEBHOOK_URL")
	cfg.GoogleChatWebhook = ""
	result, err = NormalizeAndValidate(cfg)
	if err != nil {
		t.Fatalf("NormalizeAndValidate failed: %v", err)
	}

	if result.GoogleChatWebhook != "" {
		t.Errorf("Expected empty GoogleChatWebhook, got: %q", result.GoogleChatWebhook)
	}
}
