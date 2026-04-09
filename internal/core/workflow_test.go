package core

import "testing"

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ready-for-review", "in-review"},
		{"review", "in-review"},
		{"building", "in-build"},
		{"reviewing", "in-review"},
		{"complete", "done"},
		{"completed", "done"},
		{"in-build", "in-build"},
		{"ready-for-build", "ready-for-build"},
		{"unknown-status", "unknown-status"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeStatus(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestWorkflowIsKnownStatus(t *testing.T) {
	w := DefaultWorkflow
	if !w.IsKnownStatus("in-build") {
		t.Error("in-build should be known")
	}
	if w.IsKnownStatus("nonexistent") {
		t.Error("nonexistent should not be known")
	}
}

func TestWorkflowIsTerminal(t *testing.T) {
	w := DefaultWorkflow
	if !w.IsTerminal("done") {
		t.Error("done should be terminal")
	}
	if !w.IsTerminal("archived") {
		t.Error("archived should be terminal")
	}
	if w.IsTerminal("in-build") {
		t.Error("in-build should not be terminal")
	}
}

func TestWorkflowIsActive(t *testing.T) {
	w := DefaultWorkflow
	for _, s := range []string{"in-build", "in-review", "rework", "blocked"} {
		if !w.IsActive(s) {
			t.Errorf("%s should be active", s)
		}
	}
	for _, s := range []string{"intake", "shaping", "ready-for-build", "done", "archived"} {
		if w.IsActive(s) {
			t.Errorf("%s should not be active", s)
		}
	}
}

func TestWorkflowIsValidTransition(t *testing.T) {
	w := DefaultWorkflow

	valid := []struct{ from, to string }{
		{"intake", "shaping"},
		{"shaping", "ready-for-build"},
		{"ready-for-build", "in-build"},
		{"in-build", "in-review"},
		{"in-build", "blocked"},
		{"in-review", "done"},
		{"in-review", "rework"},
		{"rework", "in-build"},
		{"done", "archived"},
	}
	for _, tt := range valid {
		if !w.IsValidTransition(tt.from, tt.to) {
			t.Errorf("transition %s -> %s should be valid", tt.from, tt.to)
		}
	}

	invalid := []struct{ from, to string }{
		{"intake", "in-build"},
		{"ready-for-build", "done"},
		{"in-build", "done"},
		{"blocked", "done"},
	}
	for _, tt := range invalid {
		if w.IsValidTransition(tt.from, tt.to) {
			t.Errorf("transition %s -> %s should be invalid", tt.from, tt.to)
		}
	}
}
