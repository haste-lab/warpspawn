package core

import (
	"fmt"
	"strings"
)

// RoleCapabilities defines what a role can and cannot edit.
type RoleCapabilities struct {
	MayEdit    []string
	MayNotEdit []string
}

// DefaultRoleCapabilities maps role names to their file edit boundaries.
var DefaultRoleCapabilities = map[string]RoleCapabilities{
	"builder": {
		MayEdit:    []string{"app/**", "src/**", "tasks/*", "status/*"},
		MayNotEdit: []string{"backlog/*", "reviews/**", "docs/*"},
	},
	"reviewer-qa": {
		MayEdit:    []string{"reviews/*"},
		MayNotEdit: []string{"app/**", "src/**", "backlog/*"},
	},
	"mission-control": {
		MayEdit:    []string{"status/*", "backlog/*", "tasks/*", "docs/decision-log.md"},
		MayNotEdit: []string{"app/**", "src/**"},
	},
	"architect": {
		MayEdit:    []string{"docs/architecture-note.md", "docs/decision-log.md", "tasks/*"},
		MayNotEdit: []string{"app/**", "src/**", "reviews/**"},
	},
	"ux": {
		MayEdit:    []string{"docs/ux-spec.md", "tasks/*"},
		MayNotEdit: []string{"app/**", "src/**", "reviews/**"},
	},
}

// ModelTierConfig maps task model tiers to provider/model selections.
type ModelTierConfig struct {
	AgentID string // which agent config to use
	Label   string // human-readable label
}

// InferModelTier determines the appropriate model tier from task properties.
func InferModelTier(task *Task) ModelTierConfig {
	// Explicit override
	if task.ModelTier == "light" {
		return ModelTierConfig{AgentID: "builder-light", Label: "Builder (light)"}
	}

	sourceCount := len(task.SourceFiles)
	criteriaCount := len(task.AcceptanceCriteria)
	objectiveLen := len(task.Objective)
	isGreenfield := false
	for _, f := range task.SourceFiles {
		if strings.Contains(f, "greenfield") || strings.Contains(f, "(empty)") {
			isGreenfield = true
			break
		}
	}

	// Light: small scope, few files, simple criteria, existing code
	if sourceCount <= 2 && criteriaCount <= 4 && objectiveLen < 200 && !isGreenfield {
		return ModelTierConfig{AgentID: "builder-light", Label: "Builder (light)"}
	}
	if sourceCount <= 1 && criteriaCount <= 3 {
		return ModelTierConfig{AgentID: "builder-light", Label: "Builder (light)"}
	}

	return ModelTierConfig{AgentID: "builder", Label: "Builder"}
}

// BuildBuilderPrompt creates a lean prompt for the Builder role.
func BuildBuilderPrompt(task *Task) (system string, user string) {
	caps := DefaultRoleCapabilities["builder"]

	system = "You are Builder, an autonomous agent that implements software tasks. " +
		"Execute the task precisely. Use tools to read/write files and run commands. " +
		"When done, call task_complete with a summary."

	var sections []string
	sections = append(sections, "# Builder Task")
	sections = append(sections, "")
	sections = append(sections, fmt.Sprintf("You MAY edit: %s. You must NOT edit: %s.",
		strings.Join(caps.MayEdit, "; "), strings.Join(caps.MayNotEdit, "; ")))
	sections = append(sections, "")
	sections = append(sections, "Before finishing, update the task file: fill Implementation Notes, Validation (with evidence), Known Limitations, Handoff. Set Status to `in-review`. Check off satisfied acceptance criteria.")
	sections = append(sections, "")
	sections = append(sections, task.RawText)

	user = strings.Join(sections, "\n")
	return
}

// BuildReviewerPrompt creates a lean prompt for the Reviewer/QA role.
func BuildReviewerPrompt(task *Task) (system string, user string) {
	caps := DefaultRoleCapabilities["reviewer-qa"]

	system = "You are Reviewer/QA, an autonomous agent that validates completed work. " +
		"Check the task against its acceptance criteria. Inspect changed files. " +
		"Write a review artifact. When done, call task_complete with your verdict."

	var sections []string
	sections = append(sections, "# Review Task")
	sections = append(sections, "")
	sections = append(sections, fmt.Sprintf("You MAY edit: %s. You must NOT edit: %s. Do NOT implement missing work — reject instead.",
		strings.Join(caps.MayEdit, "; "), strings.Join(caps.MayNotEdit, "; ")))
	sections = append(sections, "")
	sections = append(sections, "Review the task below against its acceptance criteria. Inspect the changed files. Verify validation evidence. Write `reviews/REVIEW-<taskId>.md` with: Metadata (Review ID, Task ID, Reviewer, Outcome, Date), Acceptance Criteria Result, Defects, Required Rework, Final Recommendation. Outcome must be: approved, rejected, or blocked.")
	sections = append(sections, "")
	sections = append(sections, "If a review handoff exists, read it from `status/reviewer-qa-handoff.md`.")
	sections = append(sections, "")
	sections = append(sections, task.RawText)

	user = strings.Join(sections, "\n")
	return
}
