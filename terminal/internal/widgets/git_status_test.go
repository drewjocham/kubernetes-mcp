package widgets

import "testing"

func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedBranch string
		expectedSync   string
		expectedDirty  int
	}{
		{
			name:           "Success - clean branch no upstream info",
			input:          "## adding_cli_to_mcp\n",
			expectedBranch: "adding_cli_to_mcp",
			expectedDirty:  0,
		},
		{
			name:           "Success - ahead and behind info",
			input:          "## feat-x...origin/feat-x [ahead 2, behind 1]\n M terminal/readme.md\n?? terminal/new.txt\n",
			expectedBranch: "feat-x",
			expectedSync:   "ahead 2, behind 1",
			expectedDirty:  2,
		},
		{
			name:           "Success - detached or unknown format still returns branch segment",
			input:          "## HEAD (no branch)\n M a.go\n",
			expectedBranch: "HEAD (no branch)",
			expectedDirty:  1,
		},
		{
			name:          "Success - empty output defaults",
			input:         "",
			expectedDirty: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			branch, syncInfo, dirty := parseGitStatus(tc.input)
			if branch != tc.expectedBranch {
				t.Fatalf("expected branch %q, got %q", tc.expectedBranch, branch)
			}
			if syncInfo != tc.expectedSync {
				t.Fatalf("expected sync info %q, got %q", tc.expectedSync, syncInfo)
			}
			if dirty != tc.expectedDirty {
				t.Fatalf("expected dirty %d, got %d", tc.expectedDirty, dirty)
			}
		})
	}
}
