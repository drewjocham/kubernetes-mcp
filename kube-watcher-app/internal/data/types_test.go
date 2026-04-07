package data

import (
	"testing"
	"time"
)

func TestAlertRecord_SetLifecycleStage(t *testing.T) {
	tests := []struct {
		name             string
		initialRecord    AlertRecord
		stage            string
		investigatedBy   string
		wantStage        string
		wantInvestigator string
		wantResolvedAt   bool // whether ResolvedAt should be set
	}{
		{
			name:             "Set to investigating with investigator",
			initialRecord:    AlertRecord{},
			stage:            "investigating",
			investigatedBy:   "operator1",
			wantStage:        "investigating",
			wantInvestigator: "operator1",
			wantResolvedAt:   false,
		},
		{
			name:             "Set to resolved should set timestamp",
			initialRecord:    AlertRecord{},
			stage:            "resolved",
			investigatedBy:   "operator2",
			wantStage:        "resolved",
			wantInvestigator: "operator2",
			wantResolvedAt:   true,
		},
		{
			name:             "Empty investigator does not overwrite existing",
			initialRecord:    AlertRecord{InvestigatedBy: "previous"},
			stage:            "investigating",
			investigatedBy:   "",
			wantStage:        "investigating",
			wantInvestigator: "previous",
			wantResolvedAt:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := tt.initialRecord
			record.SetLifecycleStage(tt.stage, tt.investigatedBy)

			if record.LifecycleStage != tt.wantStage {
				t.Errorf("LifecycleStage = %q, want %q", record.LifecycleStage, tt.wantStage)
			}
			if record.InvestigatedBy != tt.wantInvestigator {
				t.Errorf("InvestigatedBy = %q, want %q", record.InvestigatedBy, tt.wantInvestigator)
			}
			if tt.wantResolvedAt {
				if record.ResolvedAt.IsZero() {
					t.Error("ResolvedAt is zero, expected to be set")
				}
				// Ensure it's recent (within 1 second)
				if time.Since(record.ResolvedAt) > time.Second {
					t.Errorf("ResolvedAt is too far in the past: %v", record.ResolvedAt)
				}
			} else {
				if !record.ResolvedAt.IsZero() {
					t.Errorf("ResolvedAt should be zero, got %v", record.ResolvedAt)
				}
			}
		})
	}
}

func TestAlertRecord_MarkAsResolved(t *testing.T) {
	record := AlertRecord{}
	record.MarkAsResolved("admin", "fixed by restart")

	if record.LifecycleStage != "resolved" {
		t.Errorf("LifecycleStage = %q, want resolved", record.LifecycleStage)
	}
	if record.InvestigatedBy != "admin" {
		t.Errorf("InvestigatedBy = %q, want admin", record.InvestigatedBy)
	}
	if record.ResolutionNotes != "fixed by restart" {
		t.Errorf("ResolutionNotes = %q, want 'fixed by restart'", record.ResolutionNotes)
	}
	if record.ResolvedAt.IsZero() {
		t.Error("ResolvedAt is zero, expected to be set")
	}
}
