package githubapp

import (
	"fmt"
	"time"
)

type State string

const (
	StateSelected   State = "selected"
	StateCheckingAI State = "checking_ai_configuration"
	StateWaitingAI  State = "waiting_ai_configuration"
	StateRunQueued  State = "run_queued"
	StateRunRunning State = "run_running"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
)

var validStates = map[State]bool{
	StateSelected: true, StateCheckingAI: true, StateWaitingAI: true,
	StateRunQueued: true, StateRunRunning: true,
	StateCompleted: true, StateFailed: true, StateCancelled: true,
}

func (s State) Validate() error {
	if !validStates[s] {
		return fmt.Errorf("invalid repository import state %q", s)
	}
	return nil
}

func (s State) Terminal() bool {
	return s == StateCompleted || s == StateFailed || s == StateCancelled
}

type Installation struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"-"`
	GitHubInstallationID int64      `json:"installationId"`
	AccountID            int64      `json:"accountId"`
	AccountLogin         string     `json:"account"`
	AccountType          string     `json:"accountType"`
	TargetType           string     `json:"targetType"`
	Status               string     `json:"status"`
	SuspendedAt          *time.Time `json:"suspendedAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type Repository struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"-"`
	InstallationID string    `json:"-"`
	GitHubID       int64     `json:"githubId"`
	Name           string    `json:"name"`
	FullName       string    `json:"fullName"`
	URL            string    `json:"url"`
	DefaultBranch  string    `json:"defaultBranch"`
	Private        bool      `json:"private"`
	Archived       bool      `json:"archived"`
	Selected       bool      `json:"selected"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Step mirrors one GitHub Actions job step, as reported by the workflow_job
// webhook and the run jobs API. Order, timing, and conclusions come straight
// from GitHub; UiGraph never synthesises them.
type Step struct {
	JobID       int64      `json:"jobId"`
	JobName     string     `json:"jobName"`
	Number      int        `json:"number"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type Import struct {
	ID                     string     `json:"id"`
	OrgID                  string     `json:"-"`
	RepositoryID           string     `json:"repositoryId"`
	Repository             Repository `json:"repository"`
	TeamID                 string     `json:"teamId"`
	TeamName               string     `json:"team"`
	Status                 State      `json:"status"`
	Steps                  []Step     `json:"steps"`
	Branch                 string     `json:"branch"`
	RunID                  int64      `json:"-"`
	RunURL                 string     `json:"runUrl,omitempty"`
	PRURL                  string     `json:"prUrl,omitempty"`
	MissingAIConfiguration []string   `json:"missingAIConfiguration"`
	Error                  string     `json:"error,omitempty"`
	ServiceID              string     `json:"serviceId,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	RunStartedAt           *time.Time `json:"runStartedAt,omitempty"`
	RunCompletedAt         *time.Time `json:"runCompletedAt,omitempty"`
	CompletedAt            *time.Time `json:"completedAt,omitempty"`
}

type Job struct {
	ID          string
	OrgID       string
	ImportID    string
	Kind        string
	Attempts    int
	MaxAttempts int
}

const (
	JobStart  = "start"
	JobOpenPR = "open_pr"
)
