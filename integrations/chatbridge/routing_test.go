package chatbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectIncidentKind(t *testing.T) {
	tests := []struct {
		name string
		cfg  TriggerConfig
		text string
		want IncidentKind
		ok   bool
	}{
		{
			name: "MatchesKubernetesKeywordWithPrefix",
			cfg: TriggerConfig{
				RequirePrefixes:    []string{"@oz", "/investigate"},
				KubernetesKeywords: []string{"kubernetes", "pod", "node"},
			},
			text: "@oz pod crashloop in cluster",
			want: IncidentKubernetes,
			ok:   true,
		},
		{
			name: "MatchesSolaceKeyword",
			cfg: TriggerConfig{
				SolaceKeywords: []string{"solace", "queue"},
			},
			text: "solace queue backlog is increasing",
			want: IncidentSolace,
			ok:   true,
		},
		{
			name: "IgnoredWhenMissingPrefix",
			cfg: TriggerConfig{
				RequirePrefixes:    []string{"@oz"},
				KubernetesKeywords: []string{"k8s"},
			},
			text: "k8s issue",
			want: "",
			ok:   false,
		},
		{
			name: "IgnoredNoMatchingKeyword",
			cfg: TriggerConfig{
				RequirePrefixes:    []string{"@oz"},
				KubernetesKeywords: []string{"k8s"},
				SolaceKeywords:     []string{"solace"},
			},
			text: "@oz hello team",
			want: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DetectIncidentKind(tt.cfg, tt.text)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
