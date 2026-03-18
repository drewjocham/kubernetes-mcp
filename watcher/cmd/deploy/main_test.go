package main

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidate_Table(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config
		wantErrPart string
	}{
		{
			name: "valid deploy derives defaults from cluster",
			cfg: config{
				action:             "deploy",
				target:             "kube",
				clusterName:        "Dev_Cluster-01",
				prometheusEndpoint: "http://prometheus:9090/metrics",
				dockerDataDir:      defaultDataDir,
				dockerConfigPath:   defaultDockerConfig,
			},
		},
		{
			name: "missing cluster name",
			cfg: config{
				action:             "deploy",
				target:             "kube",
				prometheusEndpoint: "http://prometheus:9090/metrics",
			},
			wantErrPart: "--cluster-name is required",
		},
		{
			name: "deploy requires prometheus endpoint",
			cfg: config{
				action:      "deploy",
				target:      "kube",
				clusterName: "prod",
			},
			wantErrPart: "--prometheus-endpoint is required for deploy",
		},
		{
			name: "cleanup does not require prometheus endpoint",
			cfg: config{
				action:      "cleanup",
				target:      "kube",
				clusterName: "prod",
			},
		},
		{
			name: "invalid action",
			cfg: config{
				action:      "bad",
				target:      "kube",
				clusterName: "prod",
			},
			wantErrPart: "unsupported action",
		},
		{
			name: "invalid target",
			cfg: config{
				action:      "deploy",
				target:      "bad",
				clusterName: "prod",
			},
			wantErrPart: "unsupported target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAndValidate(tt.cfg)
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

			if tt.cfg.action == "deploy" && tt.cfg.clusterName != "" {
				if got.name == "" {
					t.Fatalf("expected derived deployment name, got empty")
				}
				if !strings.Contains(got.name, "kube-anomaly-detection") {
					t.Fatalf("expected derived name to include base app name, got %q", got.name)
				}
				if got.dockerDataDir == defaultDataDir {
					t.Fatalf("expected dockerDataDir to be cluster-specific, got default %q", got.dockerDataDir)
				}
				if got.dockerConfigPath == defaultDockerConfig {
					t.Fatalf("expected dockerConfigPath to be cluster-specific, got default %q", got.dockerConfigPath)
				}
				if got.pvcName == "" {
					t.Fatalf("expected default pvc name to be set")
				}
				if got.pvcSize == "" {
					t.Fatalf("expected default pvc size to be set")
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
			got := sanitizeForResourceName(tt.in)
			if got != tt.want {
				t.Fatalf("sanitizeForResourceName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestKubeManifest_UsesSecretAndPVC(t *testing.T) {
	cfg := config{
		action:             "deploy",
		target:             "kube",
		clusterName:        "dev-cluster",
		name:               "kube-anomaly-detection-dev-cluster",
		namespace:          "kubewatcher",
		image:              "kube-anomaly-detection:latest",
		prometheusEndpoint: "http://prometheus.monitoring.svc:9090/metrics",
		pvcName:            "kube-anomaly-detection-dev-cluster-data",
		pvcSize:            "10Gi",
	}

	manifest := kubeManifest(cfg)

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
