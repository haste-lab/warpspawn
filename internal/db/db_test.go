package db

import (
	"testing"
	"time"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer db.Close()

	// Verify schema version
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("schema version query failed: %v", err)
	}
	if version != 1 {
		t.Errorf("schema version = %d, want 1", version)
	}
}

func TestReopenIdempotent(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first open failed: %v", err)
	}
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	defer db2.Close()

	var version int
	db2.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if version != 1 {
		t.Errorf("schema version = %d after reopen, want 1", version)
	}
}

func TestUpsertProjectAndSummary(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.UpsertProject("proj-1", "/home/test/proj-1", "Test Project", "active", "ready for build", "epoch-1")
	if err != nil {
		t.Fatalf("upsert project: %v", err)
	}

	summaries, err := db.GetProjectSummaries()
	if err != nil {
		t.Fatalf("get summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Name != "Test Project" {
		t.Errorf("name = %q", summaries[0].Name)
	}
}

func TestUpsertTask(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.UpsertProject("proj-1", "/test", "Test", "active", "", "")
	err = db.UpsertTask("proj-1", "TASK-001", "/test/tasks/t1.md", "Build MVP", "ready-for-build", "P0", "Builder", "standard")
	if err != nil {
		t.Fatalf("upsert task: %v", err)
	}

	// Verify via summary
	summaries, _ := db.GetProjectSummaries()
	if summaries[0].TotalTasks != 1 {
		t.Errorf("total tasks = %d, want 1", summaries[0].TotalTasks)
	}
}

func TestRecordRun(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.UpsertProject("proj-1", "/test", "Test", "active", "", "")

	id, err := db.RecordRun(RunRecord{
		ProjectID:    "proj-1",
		TaskID:       "TASK-001",
		Role:         "builder",
		Model:        "gpt-5.4-mini",
		Provider:     "openai",
		InputTokens:  1500,
		OutputTokens: 800,
		CostUSD:      0.004,
		ToolCalls:    5,
		DurationMS:   12000,
		Status:       "completed",
		Summary:      "Built the MVP",
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		CompletedAt:  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("record run: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero run ID")
	}
}

func TestGetDailyCost(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.UpsertProject("proj-1", "/test", "Test", "active", "", "")
	db.RecordRun(RunRecord{
		ProjectID: "proj-1", Role: "builder", Model: "gpt-5.4-mini", Provider: "openai",
		InputTokens: 1000, OutputTokens: 500, CostUSD: 0.25,
		Status: "completed", StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	db.RecordRun(RunRecord{
		ProjectID: "proj-1", Role: "reviewer", Model: "gpt-5.4-mini", Provider: "openai",
		InputTokens: 500, OutputTokens: 200, CostUSD: 0.10,
		Status: "completed", StartedAt: time.Now().UTC().Format(time.RFC3339),
	})

	cost, err := db.GetDailyCost()
	if err != nil {
		t.Fatalf("get daily cost: %v", err)
	}
	if cost < 0.34 || cost > 0.36 {
		t.Errorf("daily cost = %f, want ~0.35", cost)
	}
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.UpsertProject("proj-1", "/test", "Test", "active", "", "")

	if err := db.Backup(); err != nil {
		t.Fatalf("backup failed: %v", err)
	}
}
