package cmd

import "testing"

func TestNewViewCmd_RegistersFeatureParityCommands(t *testing.T) {
	tests := []struct {
		name         string
		commandNames []string
	}{
		{
			name: "ContainsRequestedFeatureCommands",
			commandNames: []string{
				"node-status",
				"pod-resources",
				"namespaces",
				"pod-logs",
				"cluster-analysis",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewCmd := newViewCmd()
			registered := map[string]struct{}{}
			for _, command := range viewCmd.Commands() {
				registered[command.Name()] = struct{}{}
			}

			for _, commandName := range tt.commandNames {
				if _, ok := registered[commandName]; !ok {
					t.Fatalf("expected command %q to be registered", commandName)
				}
			}
		})
	}
}
