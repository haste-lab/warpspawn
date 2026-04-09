package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateTaskStatusInFile(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("# Task\n- Status: ready-for-build\n- Title: Test\n"), 0644)

	updateTaskStatusInFile(taskPath, "in-build")

	data, _ := os.ReadFile(taskPath)
	if got := ExtractMetadataValue(string(data), "Status"); got != "in-build" {
		t.Errorf("status = %q, want in-build", got)
	}
}

func TestUpdateProjectStage(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "status"), 0755)
	statePath := filepath.Join(dir, "status", "project-state.md")
	os.WriteFile(statePath, []byte("# State\n- Current Stage: ready for build\n"), 0644)

	updateProjectStage(dir, "in progress")

	data, _ := os.ReadFile(statePath)
	if got := ExtractMetadataValue(string(data), "Current Stage"); got != "in progress" {
		t.Errorf("stage = %q, want 'in progress'", got)
	}
}

func TestAppendToTaskSection(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("# Task\n## Validation\npending\n## Handoff\npending\n"), 0644)

	appendToTaskSection(taskPath, "Validation", "All tests passed.")

	data, _ := os.ReadFile(taskPath)
	content := string(data)
	if ExtractSection(content, "Validation") != "All tests passed." {
		t.Errorf("validation section not updated: %q", ExtractSection(content, "Validation"))
	}
	// Handoff should still be pending
	if ExtractSection(content, "Handoff") != "pending" {
		t.Errorf("handoff should be unchanged: %q", ExtractSection(content, "Handoff"))
	}
}

func TestAppendToTaskSection_NoPlaceholder(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("# Task\n## Validation\nAlready filled.\n"), 0644)

	appendToTaskSection(taskPath, "Validation", "Should not overwrite.")

	data, _ := os.ReadFile(taskPath)
	if got := ExtractSection(string(data), "Validation"); got != "Already filled." {
		t.Errorf("should not overwrite existing content: %q", got)
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := atomicWrite(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want 'hello'", string(data))
	}

	// No temp file should remain
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

func TestAtomicWrite_FailsOnBadPath(t *testing.T) {
	err := atomicWrite("/nonexistent/dir/file.txt", []byte("data"))
	if err == nil {
		t.Error("expected error for bad path")
	}
}
