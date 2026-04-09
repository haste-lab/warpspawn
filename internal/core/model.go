package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Task represents a parsed task file.
type Task struct {
	Path                string
	TaskID              string
	Title               string
	Status              string
	Priority            string
	OwnerRole           string
	DependsOn           string
	SourceFiles         []string
	ModelTier           string
	Objective           string
	InScope             string
	OutOfScope          string
	AcceptanceCriteria  []string
	Constraints         string
	ImplementationNotes string
	Validation          string
	Handoff             string
	RawText             string
}

// BacklogItem represents a row from the backlog table.
type BacklogItem struct {
	ID        string
	Title     string
	Status    string
	Priority  string
	OwnerRole string
	DependsOn string
	Source    string
	Notes     string
}

// Review represents a parsed review file.
type Review struct {
	Path                string
	ReviewID            string
	TaskID              string
	Reviewer            string
	Outcome             string
	Date                string
	FinalRecommendation string
	RequiredRework      string
	RawText             string
}

// ProjectState represents the project control state.
type ProjectState struct {
	Lifecycle                  string
	PickupEnabled              bool
	EscalationDeliveryEnabled  bool
	CurrentEpoch               string
	CurrentStage               string
	StatePath                  string
	RawText                    string
}

// ParseTaskFile parses a task markdown file into a Task struct.
func ParseTaskFile(filePath string) (Task, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Task{}, err
	}
	text := string(data)

	sourceFilesRaw := ExtractMetadataValue(text, "Source Files")
	var sourceFiles []string
	for _, f := range strings.Split(sourceFilesRaw, ";") {
		f = strings.TrimSpace(f)
		if f != "" {
			sourceFiles = append(sourceFiles, f)
		}
	}

	modelTier := strings.ToLower(strings.TrimSpace(ExtractMetadataValue(text, "Model Tier")))
	if modelTier == "" {
		modelTier = "standard"
	}

	status := NormalizeStatus(strings.ToLower(strings.TrimSpace(ExtractMetadataValue(text, "Status"))))

	return Task{
		Path:                filePath,
		TaskID:              ExtractMetadataValue(text, "Task ID"),
		Title:               ExtractMetadataValue(text, "Title"),
		Status:              status,
		Priority:            ExtractMetadataValue(text, "Priority"),
		OwnerRole:           ExtractMetadataValue(text, "Owner Role"),
		DependsOn:           ExtractMetadataValue(text, "Depends On"),
		SourceFiles:         sourceFiles,
		ModelTier:           modelTier,
		Objective:           ExtractSection(text, "Objective"),
		InScope:             ExtractSection(text, "In Scope"),
		OutOfScope:          ExtractSection(text, "Out of Scope"),
		AcceptanceCriteria:  ExtractChecklistItems(text, "Acceptance Criteria"),
		Constraints:         ExtractSection(text, "Constraints"),
		ImplementationNotes: ExtractSection(text, "Implementation Notes"),
		Validation:          ExtractSection(text, "Validation"),
		Handoff:             ExtractSection(text, "Handoff"),
		RawText:             text,
	}, nil
}

// ParseBacklog parses the backlog markdown into BacklogItems.
func ParseBacklog(text string) []BacklogItem {
	rows := ParseMarkdownTableRows(text, "| ID | Title | Status |")
	var items []BacklogItem
	for _, row := range rows {
		item := BacklogItem{}
		if len(row) > 0 { item.ID = row[0] }
		if len(row) > 1 { item.Title = row[1] }
		if len(row) > 2 { item.Status = row[2] }
		if len(row) > 3 { item.Priority = row[3] }
		if len(row) > 4 { item.OwnerRole = row[4] }
		if len(row) > 5 { item.DependsOn = row[5] }
		if len(row) > 6 { item.Source = row[6] }
		if len(row) > 7 { item.Notes = row[7] }
		items = append(items, item)
	}
	return items
}

// ParseReviewFile parses a review markdown file.
func ParseReviewFile(filePath string) (Review, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Review{}, err
	}
	text := string(data)
	return Review{
		Path:                filePath,
		ReviewID:            ExtractMetadataValue(text, "Review ID"),
		TaskID:              ExtractMetadataValue(text, "Task ID"),
		Reviewer:            ExtractMetadataValue(text, "Reviewer"),
		Outcome:             ExtractMetadataValue(text, "Outcome"),
		Date:                ExtractMetadataValue(text, "Date"),
		FinalRecommendation: ExtractSection(text, "Final Recommendation"),
		RequiredRework:      ExtractSection(text, "Required Rework"),
		RawText:             text,
	}, nil
}

// LoadProjectState reads and parses the project state file.
func LoadProjectState(projectRoot string) ProjectState {
	statePath := filepath.Join(projectRoot, "status", "project-state.md")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return ProjectState{
			Lifecycle:     "active",
			PickupEnabled: true,
			EscalationDeliveryEnabled: true,
			StatePath:     statePath,
		}
	}
	text := string(data)
	lifecycle := strings.ToLower(strings.TrimSpace(ExtractMetadataValue(text, "Project Lifecycle")))
	if lifecycle == "" {
		lifecycle = "active"
	}

	return ProjectState{
		Lifecycle:                  lifecycle,
		PickupEnabled:              isEnabled(ExtractMetadataValue(text, "Autonomous Pickup")),
		EscalationDeliveryEnabled:  isEnabled(ExtractMetadataValue(text, "Escalation Delivery")),
		CurrentEpoch:               ExtractMetadataValue(text, "Current Epoch"),
		CurrentStage:               ExtractMetadataValue(text, "Current Stage"),
		StatePath:                  statePath,
		RawText:                    text,
	}
}

// ListTasks discovers and parses all task files in a project.
func ListTasks(projectRoot string) []Task {
	tasksDir := filepath.Join(projectRoot, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil
	}

	var tasks []Task
	seen := make(map[string]bool) // dedup by task ID (case-insensitive)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "README.md") {
			continue
		}
		task, err := ParseTaskFile(filepath.Join(tasksDir, name))
		if err != nil {
			continue
		}
		// Skip duplicate task IDs (case-insensitive) — keep the first one found
		idKey := strings.ToLower(task.TaskID)
		if seen[idKey] {
			continue
		}
		seen[idKey] = true
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].TaskID < tasks[j].TaskID
	})
	return tasks
}

// LoadLatestReview finds the most recent review for a given task ID.
func LoadLatestReview(projectRoot, taskID string) *Review {
	reviewsDir := filepath.Join(projectRoot, "reviews")
	entries, err := os.ReadDir(reviewsDir)
	if err != nil {
		return nil
	}

	var matches []Review
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		review, err := ParseReviewFile(filepath.Join(reviewsDir, entry.Name()))
		if err != nil {
			continue
		}
		if review.TaskID == taskID {
			matches = append(matches, review)
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by date then review ID, return latest
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Date != matches[j].Date {
			return matches[i].Date < matches[j].Date
		}
		return matches[i].ReviewID < matches[j].ReviewID
	})
	latest := matches[len(matches)-1]
	return &latest
}

func isEnabled(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return true // default to enabled if not specified
	}
	return strings.Contains(v, "enabled")
}

// PriorityRank converts priority strings to sortable integers.
func PriorityRank(priority string) int {
	switch priority {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 99
	}
}
