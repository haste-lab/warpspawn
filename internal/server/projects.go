package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/haste-lab/warpspawn/internal/config"
)

type createProjectRequest struct {
	Name     string                       `json:"name"`
	Brief    string                       `json:"brief"`
	Strategy string                       `json:"strategy"` // "defaults" or "custom"
	Roles    map[string]roleModelOverride `json:"roles,omitempty"`
}

type roleModelOverride struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Brief) == "" {
		http.Error(w, "project brief is required", http.StatusBadRequest)
		return
	}

	// Generate project ID from name or brief
	projectID := slugify(req.Name)
	if projectID == "" {
		// Use first few words of the brief
		words := strings.Fields(req.Brief)
		if len(words) > 4 {
			words = words[:4]
		}
		projectID = slugify(strings.Join(words, " "))
	}
	if projectID == "" {
		projectID = fmt.Sprintf("project-%d", time.Now().Unix())
	}

	// Determine project directory
	paths := config.DefaultPaths()
	projectRoot := filepath.Join(paths.ProjectDir, projectID)

	// Check if already exists
	if _, err := os.Stat(projectRoot); err == nil {
		// Append timestamp to make unique
		projectID = fmt.Sprintf("%s-%d", projectID, time.Now().Unix())
		projectRoot = filepath.Join(paths.ProjectDir, projectID)
	}

	slog.Info("creating project", "id", projectID, "root", projectRoot)

	// Scaffold directory structure
	dirs := []string{
		"tasks",
		"status",
		"backlog",
		"docs",
		"reviews",
		"app",
		"config",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(projectRoot, dir), 0755); err != nil {
			http.Error(w, "failed to create directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Write project brief
	briefContent := fmt.Sprintf(`# Project Brief

## Project
- Name: %s
- Owner: Mission Control
- Status: active
- Last Updated: %s

## Objective
%s

## Scope In
(to be defined during shaping)

## Scope Out
(to be defined during shaping)

## Constraints
(to be defined during shaping)
`, coalesce(req.Name, projectID), time.Now().Format("2006-01-02"), strings.TrimSpace(req.Brief))
	writeProjectFile(projectRoot, "docs/project-brief.md", briefContent)

	// Write project state
	stateContent := fmt.Sprintf(`# Project State Snapshot

- Project: %s
- Project Lifecycle: active
- Current Stage: intake
- Autonomous Pickup: enabled
- Escalation Delivery: enabled
- Current Epoch: %s
- Default Flow: Mission Control -> UX + Architect -> Builder -> Reviewer/QA -> Mission Control
`, coalesce(req.Name, projectID), projectID+"-"+time.Now().Format("2006-01-02"))
	writeProjectFile(projectRoot, "status/project-state.md", stateContent)

	// Write backlog
	backlogContent := `## Backlog

| ID | Title | Status | Priority | Owner Role | Depends On | Source | Notes |
|---|---|---|---|---|---|---|---|
`
	writeProjectFile(projectRoot, "backlog/backlog.md", backlogContent)

	// Write status log
	statusLogContent := fmt.Sprintf(`## Status Log

| Date | Item | Status | Owner Role | Summary | Blockers | Next Step |
|---|---|---|---|---|---|---|
| %s | Project created | intake | Mission Control | Project scaffolded from brief | None | Shape project scope |
`, time.Now().Format("2006-01-02"))
	writeProjectFile(projectRoot, "status/status-log.md", statusLogContent)

	// Write empty doc stubs
	writeProjectFile(projectRoot, "docs/architecture-note.md", "# Architecture Note\n\n(to be filled during shaping)\n")
	writeProjectFile(projectRoot, "docs/ux-spec.md", "# UX Specification\n\n(to be filled during shaping)\n")
	writeProjectFile(projectRoot, "docs/decision-log.md", "# Decision Log\n\n| Date | ID | Decision | Reason | Owner Role | Impact | Follow-Up |\n|---|---|---|---|---|---|---|\n")
	writeProjectFile(projectRoot, "reviews/README.md", "# Reviews\n\nReview artifacts for this project.\n")
	writeProjectFile(projectRoot, "tasks/README.md", "# Tasks\n\nTask files for this project.\n")

	// Write mission-control entry
	mcContent := fmt.Sprintf(`# Mission Control Entry Point

## Rules
- Use the project files as the source of truth
- Never implement tasks directly — delegate through the runtime
- Read status/project-state.md and backlog/backlog.md first

## Project Brief
%s
`, strings.TrimSpace(req.Brief))
	writeProjectFile(projectRoot, "mission-control.md", mcContent)

	// Write model policy if custom strategy
	if req.Strategy == "custom" && len(req.Roles) > 0 {
		var lines []string
		lines = append(lines, "# Model Policy\n")
		lines = append(lines, "## Role Mapping")
		for role, cfg := range req.Roles {
			lines = append(lines, fmt.Sprintf("- %s -> %s/%s", role, cfg.Provider, cfg.Model))
		}
		writeProjectFile(projectRoot, "config/model-policy.md", strings.Join(lines, "\n")+"\n")
	}

	// Init git
	initGit(projectRoot)

	// Index in SQLite
	if s.db != nil {
		s.db.UpsertProject(projectID, projectRoot, coalesce(req.Name, projectID), "active", "intake", "")
	}

	slog.Info("project created", "id", projectID, "root", projectRoot)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":   projectID,
		"path": projectRoot,
	})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	paths := config.DefaultPaths()

	// Try to find project path: first from SQLite, then assume managed dir
	projectRoot := ""
	if s.db != nil {
		detail, err := s.db.GetProjectDetail(projectID)
		if err == nil && detail.Path != "" {
			projectRoot = detail.Path
		}
	}
	if projectRoot == "" {
		projectRoot = filepath.Join(paths.ProjectDir, projectID)
	}

	// Determine if project is inside managed directory
	absProject, _ := filepath.Abs(projectRoot)
	absProjects, _ := filepath.Abs(paths.ProjectDir)
	insideManaged := strings.HasPrefix(absProject, absProjects+string(filepath.Separator))

	slog.Info("deleting project", "id", projectID, "root", projectRoot, "managed", insideManaged)

	// Remove from SQLite
	if s.db != nil {
		s.db.DeleteProject(projectID)
	}

	// Remove chat session from memory
	chatMu.Lock()
	delete(chatSessions, projectID)
	chatMu.Unlock()

	// Only delete files if project is inside managed directory
	// External projects (e.g. /tmp, imported) are only removed from the index
	if insideManaged {
		if _, err := os.Stat(projectRoot); err == nil {
			if err := os.RemoveAll(projectRoot); err != nil {
				slog.Error("failed to remove project directory", "error", err)
				http.Error(w, "failed to delete project files: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		slog.Info("external project — removed from index only, files preserved", "path", projectRoot)
	}

	slog.Info("project deleted", "id", projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": projectID})
}

func writeProjectFile(projectRoot, relPath, content string) {
	fullPath := filepath.Join(projectRoot, relPath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	tmpPath := fullPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	os.WriteFile(tmpPath, []byte(content), 0644)
	os.Rename(tmpPath, fullPath)
}

func initGit(projectRoot string) {
	// Only init if git is available
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return
	}
	cmd := exec.Command(gitPath, "init")
	cmd.Dir = projectRoot
	cmd.Run()

	cmd = exec.Command(gitPath, "add", "-A")
	cmd.Dir = projectRoot
	cmd.Run()

	cmd = exec.Command(gitPath, "commit", "-m", "[warpspawn] project scaffolded")
	cmd.Dir = projectRoot
	cmd.Run()
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
