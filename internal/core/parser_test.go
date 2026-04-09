package core

import "testing"

func TestExtractMetadataValue(t *testing.T) {
	text := `## Metadata
- Task ID: TASK-001
- Title: Build dashboard MVP
- Status: ready-for-build
- Priority: P0
- Owner Role: Builder
- Source Files: app/src/server.js; app/public/index.html
`
	tests := []struct {
		label    string
		expected string
	}{
		{"Task ID", "TASK-001"},
		{"Title", "Build dashboard MVP"},
		{"Status", "ready-for-build"},
		{"Priority", "P0"},
		{"Owner Role", "Builder"},
		{"Source Files", "app/src/server.js; app/public/index.html"},
		{"Nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := ExtractMetadataValue(text, tt.label)
			if got != tt.expected {
				t.Errorf("ExtractMetadataValue(%q) = %q, want %q", tt.label, got, tt.expected)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	text := `## Objective
Build a local dashboard.

## In Scope
- local app
- localhost binding

## Out of Scope
- internet exposure
`
	objective := ExtractSection(text, "Objective")
	if objective != "Build a local dashboard." {
		t.Errorf("Objective = %q", objective)
	}

	inScope := ExtractSection(text, "In Scope")
	if inScope != "- local app\n- localhost binding" {
		t.Errorf("In Scope = %q", inScope)
	}

	missing := ExtractSection(text, "Nonexistent")
	if missing != "" {
		t.Errorf("Nonexistent should be empty, got %q", missing)
	}
}

func TestExtractChecklistItems(t *testing.T) {
	text := `## Acceptance Criteria
- [x] app can be started locally
- [ ] dashboard displays CPU
- [ ] data refreshes automatically
- some non-checklist line
`
	items := ExtractChecklistItems(text, "Acceptance Criteria")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0] != "- [x] app can be started locally" {
		t.Errorf("item 0 = %q", items[0])
	}
}

func TestIsPlaceholderSection(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", true},
		{"  ", true},
		{"pending", true},
		{"Pending", true},
		{"- pending", true},
		{"- Pending", true},
		{"actual content here", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsPlaceholderSection(tt.input); got != tt.expected {
				t.Errorf("IsPlaceholderSection(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMarkdownTableRows(t *testing.T) {
	markdown := `## Backlog

| ID | Title | Status | Priority |
|---|---|---|---|
| TASK-001 | Build MVP | ready-for-build | P0 |
| TASK-002 | Add tests | intake | P1 |
`

	rows := ParseMarkdownTableRows(markdown, "| ID | Title | Status |")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "TASK-001" {
		t.Errorf("row 0 col 0 = %q", rows[0][0])
	}
	if rows[1][2] != "intake" {
		t.Errorf("row 1 col 2 = %q", rows[1][2])
	}
}
