package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesEditPattern(t *testing.T) {
	tests := []struct {
		file    string
		pattern string
		match   bool
	}{
		{"app/src/main.go", "app/**", true},
		{"app/src/deep/file.go", "app/**", true},
		{"tasks/TASK-001.md", "tasks/*", true},
		{"tasks/sub/file.md", "tasks/*", false}, // /* is one level only
		{"docs/decision-log.md", "docs/decision-log.md", true},
		{"docs/other.md", "docs/decision-log.md", false},
		{"reviews/REVIEW-001.md", "reviews/*", true},
		{"backlog/backlog.md", "app/**", false},
		{"status/next-action.md", "status/*", true},
		{"status/role-runs/run1/out.log", "status/*", false}, // /* is one level
		{"src/main.go", "src/**", true},
	}

	for _, tt := range tests {
		t.Run(tt.file+"_"+tt.pattern, func(t *testing.T) {
			got := matchesEditPattern(tt.file, tt.pattern)
			if got != tt.match {
				t.Errorf("matchesEditPattern(%q, %q) = %v, want %v", tt.file, tt.pattern, got, tt.match)
			}
		})
	}
}

func TestMatchesEditPatternWithComment(t *testing.T) {
	// Patterns with comments should be stripped
	if !matchesEditPattern("status/next.md", "status/* implementation artifacts") {
		t.Error("should match with comment stripped")
	}
	if !matchesEditPattern("tasks/TASK-001.md", "tasks/* during shaping") {
		t.Error("should match with 'during' comment stripped")
	}
}

func TestCreateAndDetectChanges(t *testing.T) {
	dir := t.TempDir()

	// Create initial state
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)

	// Take snapshot
	pre := CreateManifest(dir)
	if len(pre) != 2 {
		t.Fatalf("expected 2 files in manifest, got %d", len(pre))
	}

	// Make changes
	os.WriteFile(filepath.Join(dir, "app", "main.go"), []byte("package main\n// updated"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "new.go"), []byte("package main"), 0644)
	os.Remove(filepath.Join(dir, "README.md"))

	changed, added, removed := DetectChanges(dir, pre)

	if len(changed) != 1 || changed[0] != "app/main.go" {
		t.Errorf("changed = %v, want [app/main.go]", changed)
	}
	if len(added) != 1 || added[0] != "app/new.go" {
		t.Errorf("added = %v, want [app/new.go]", added)
	}
	if len(removed) != 1 || removed[0] != "README.md" {
		t.Errorf("removed = %v, want [README.md]", removed)
	}
}

func TestValidateRoleChanges(t *testing.T) {
	dir := t.TempDir()

	// Create initial state
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "tasks"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "tasks", "TASK-001.md"), []byte("# Task"), 0644)

	pre := CreateManifest(dir)

	// Builder modifies app/ (allowed) and creates backlog/item.md (not allowed)
	os.WriteFile(filepath.Join(dir, "app", "main.go"), []byte("package main\n// built"), 0644)
	os.MkdirAll(filepath.Join(dir, "backlog"), 0755)
	os.WriteFile(filepath.Join(dir, "backlog", "item.md"), []byte("# Bad"), 0644)

	result := ValidateRoleChanges(dir, pre, []string{"app/**", "tasks/*"})
	if result.Skipped {
		t.Fatal("should not be skipped")
	}
	if len(result.Authorized) != 1 {
		t.Errorf("expected 1 authorized, got %d: %v", len(result.Authorized), result.Authorized)
	}
	if len(result.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d: %v", len(result.Violations), result.Violations)
	}
	if result.Violations[0].Path != "backlog/item.md" {
		t.Errorf("expected violation on backlog/item.md, got %s", result.Violations[0].Path)
	}
}

func TestValidateRoleChangesNilManifest(t *testing.T) {
	result := ValidateRoleChanges("/tmp", nil, []string{"app/**"})
	if !result.Skipped {
		t.Error("should be skipped with nil manifest")
	}
}

func TestSaveAndLoadManifest(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "status"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)

	manifest := CreateManifest(dir)
	SaveManifest(dir, manifest)

	loaded := LoadManifest(dir)
	if loaded == nil {
		t.Fatal("loaded manifest is nil")
	}
	if _, ok := loaded["file.txt"]; !ok {
		t.Error("file.txt not in loaded manifest")
	}
}
