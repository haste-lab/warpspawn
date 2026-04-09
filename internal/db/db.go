package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn    *sql.DB
	dbPath  string
}

// Open creates or opens the SQLite database.
func Open(dataDir string) (*DB, error) {
	os.MkdirAll(dataDir, 0755)
	dbPath := filepath.Join(dataDir, "warpspawn.db")

	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for concurrent reads during writes
	conn.Exec("PRAGMA journal_mode=WAL")
	conn.Exec("PRAGMA busy_timeout=5000")

	db := &DB{conn: conn, dbPath: dbPath}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Backup copies the database to a backup file.
func (db *DB) Backup() error {
	backupPath := db.dbPath + ".bak"
	data, err := os.ReadFile(db.dbPath)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, data, 0644)
}

func (db *DB) migrate() error {
	// Create schema version table
	_, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`)
	if err != nil {
		return err
	}

	var version int
	err = db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return err
	}

	migrations := []string{
		migration001,
	}

	for i := version; i < len(migrations); i++ {
		tx, err := db.conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", i+1); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

const migration001 = `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	lifecycle TEXT NOT NULL DEFAULT 'active',
	current_stage TEXT,
	current_epoch TEXT,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
	id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	path TEXT NOT NULL,
	title TEXT,
	status TEXT NOT NULL,
	priority TEXT,
	owner_role TEXT,
	model_tier TEXT DEFAULT 'standard',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, id),
	FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE TABLE IF NOT EXISTS runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	task_id TEXT,
	role TEXT NOT NULL,
	model TEXT NOT NULL,
	provider TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	tool_calls INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	summary TEXT,
	started_at TEXT NOT NULL,
	completed_at TEXT,
	FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE TABLE IF NOT EXISTS reviews (
	id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	outcome TEXT,
	reviewer TEXT,
	date TEXT,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, id),
	FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_started ON runs(started_at);
`

// RecordRun inserts a completed agent run.
func (db *DB) RecordRun(run RunRecord) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO runs (project_id, task_id, role, model, provider, input_tokens, output_tokens, cost_usd, tool_calls, duration_ms, status, summary, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ProjectID, run.TaskID, run.Role, run.Model, run.Provider,
		run.InputTokens, run.OutputTokens, run.CostUSD,
		run.ToolCalls, run.DurationMS, run.Status, run.Summary,
		run.StartedAt, run.CompletedAt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// RunRecord represents a completed agent run.
type RunRecord struct {
	ProjectID    string
	TaskID       string
	Role         string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	ToolCalls    int
	DurationMS   int64
	Status       string
	Summary      string
	StartedAt    string
	CompletedAt  string
}

// UpsertProject inserts or updates a project in the index.
func (db *DB) UpsertProject(id, path, name, lifecycle, stage, epoch string) error {
	_, err := db.conn.Exec(`
		INSERT INTO projects (id, path, name, lifecycle, current_stage, current_epoch, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path, name=excluded.name, lifecycle=excluded.lifecycle,
			current_stage=excluded.current_stage, current_epoch=excluded.current_epoch,
			updated_at=excluded.updated_at`,
		id, path, name, lifecycle, stage, epoch, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UpsertTask inserts or updates a task in the index.
func (db *DB) UpsertTask(projectID, taskID, path, title, status, priority, ownerRole, modelTier string) error {
	_, err := db.conn.Exec(`
		INSERT INTO tasks (id, project_id, path, title, status, priority, owner_role, model_tier, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, id) DO UPDATE SET
			path=excluded.path, title=excluded.title, status=excluded.status,
			priority=excluded.priority, owner_role=excluded.owner_role,
			model_tier=excluded.model_tier, updated_at=excluded.updated_at`,
		taskID, projectID, path, title, status, priority, ownerRole, modelTier,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetDailyCost returns the total cost for today.
func (db *DB) GetDailyCost() (float64, error) {
	today := time.Now().Format("2006-01-02")
	var cost float64
	err := db.conn.QueryRow(`
		SELECT COALESCE(SUM(cost_usd), 0) FROM runs
		WHERE started_at >= ?`, today+"T00:00:00Z",
	).Scan(&cost)
	return cost, err
}

// GetProjectSummaries returns all projects with their task counts.
func (db *DB) GetProjectSummaries() ([]ProjectSummary, error) {
	rows, err := db.conn.Query(`
		SELECT p.id, p.name, p.lifecycle, p.current_stage,
			COALESCE((SELECT COUNT(*) FROM tasks t WHERE t.project_id = p.id), 0),
			COALESCE((SELECT COUNT(*) FROM tasks t WHERE t.project_id = p.id AND t.status = 'done'), 0)
		FROM projects p
		ORDER BY p.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ProjectSummary
	for rows.Next() {
		var s ProjectSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.Lifecycle, &s.CurrentStage, &s.TotalTasks, &s.DoneTasks); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// ProjectSummary is a lightweight project view for the dashboard.
type ProjectSummary struct {
	ID           string
	Name         string
	Lifecycle    string
	CurrentStage string
	TotalTasks   int
	DoneTasks    int
}
