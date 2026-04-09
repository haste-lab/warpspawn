package core

// Workflow defines the task lifecycle as a data structure.
// The decision engine reads from this structure — no hardcoded status strings.
// v1 ships with DefaultWorkflow only. v2 will load custom workflows from YAML.

type StatusDef struct {
	Phase    string // planning, execution, closed
	Terminal bool
	Active   bool // true for statuses that represent in-flight work
}

type Transition struct {
	From string
	To   string
	Role string
}

type RoutingConfig struct {
	ActiveStatuses   []string
	ReadyStatus      string
	ShapingStatus    string
	TerminalStatuses []string
	BuilderEntry     string
	ReviewEntry      string
}

type Workflow struct {
	ID          string
	Statuses    map[string]StatusDef
	Transitions []Transition
	Routing     RoutingConfig
}

// StatusNormalizations maps common variants to canonical status names.
var StatusNormalizations = map[string]string{
	"ready-for-review": "in-review",
	"review":           "in-review",
	"building":         "in-build",
	"reviewing":        "in-review",
	"complete":         "done",
	"completed":        "done",
}

var DefaultWorkflow = Workflow{
	ID: "default",
	Statuses: map[string]StatusDef{
		"intake":          {Phase: "planning", Terminal: false, Active: false},
		"shaping":         {Phase: "planning", Terminal: false, Active: false},
		"ready-for-build": {Phase: "execution", Terminal: false, Active: false},
		"in-build":        {Phase: "execution", Terminal: false, Active: true},
		"in-review":       {Phase: "execution", Terminal: false, Active: true},
		"rework":          {Phase: "execution", Terminal: false, Active: true},
		"blocked":         {Phase: "execution", Terminal: false, Active: true},
		"done":            {Phase: "closed", Terminal: true, Active: false},
		"archived":        {Phase: "closed", Terminal: true, Active: false},
	},
	Transitions: []Transition{
		{From: "intake", To: "shaping", Role: "mission-control"},
		{From: "shaping", To: "ready-for-build", Role: "mission-control"},
		{From: "ready-for-build", To: "in-build", Role: "runtime"},
		{From: "in-build", To: "in-review", Role: "builder"},
		{From: "in-build", To: "blocked", Role: "builder"},
		{From: "in-review", To: "done", Role: "mission-control"},
		{From: "in-review", To: "rework", Role: "mission-control"},
		{From: "in-review", To: "blocked", Role: "reviewer-qa"},
		{From: "rework", To: "in-build", Role: "runtime"},
		{From: "blocked", To: "ready-for-build", Role: "mission-control"},
		{From: "done", To: "archived", Role: "mission-control"},
	},
	Routing: RoutingConfig{
		ActiveStatuses:   []string{"in-build", "in-review", "rework", "blocked"},
		ReadyStatus:      "ready-for-build",
		ShapingStatus:    "shaping",
		TerminalStatuses: []string{"done", "archived"},
		BuilderEntry:     "in-build",
		ReviewEntry:      "in-review",
	},
}

// NormalizeStatus maps common status variants to canonical names.
func NormalizeStatus(status string) string {
	if canonical, ok := StatusNormalizations[status]; ok {
		return canonical
	}
	return status
}

// IsKnownStatus checks if a status exists in the workflow.
func (w *Workflow) IsKnownStatus(status string) bool {
	_, ok := w.Statuses[status]
	return ok
}

// IsTerminal checks if a status is a terminal (closed) status.
func (w *Workflow) IsTerminal(status string) bool {
	if def, ok := w.Statuses[status]; ok {
		return def.Terminal
	}
	return false
}

// IsActive checks if a status represents in-flight work.
func (w *Workflow) IsActive(status string) bool {
	if def, ok := w.Statuses[status]; ok {
		return def.Active
	}
	return false
}

// IsValidTransition checks if a status change is allowed.
func (w *Workflow) IsValidTransition(from, to string) bool {
	for _, t := range w.Transitions {
		if t.From == from && t.To == to {
			return true
		}
	}
	return false
}
