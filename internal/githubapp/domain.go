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
		return fmt.Errorf("invalid repository onboarding state %q", s)
	}
	return nil
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

type Batch struct {
	ID        string       `json:"id"`
	OrgID     string       `json:"-"`
	TeamID    string       `json:"teamId"`
	TeamName  string       `json:"team"`
	Status    string       `json:"status"`
	Items     []Onboarding `json:"repositories"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

type Onboarding struct {
	ID                     string     `json:"id"`
	OrgID                  string     `json:"-"`
	BatchID                string     `json:"-"`
	RepositoryID           string     `json:"repositoryId"`
	Repository             Repository `json:"repository"`
	TeamID                 string     `json:"teamId"`
	TeamName               string     `json:"team"`
	Status                 State      `json:"status"`
	Branch                 string     `json:"branch"`
	RunID                  int64      `json:"-"`
	RunURL                 string     `json:"runUrl,omitempty"`
	PRURL                  string     `json:"prUrl,omitempty"`
	MissingAIConfiguration []string   `json:"missingAIConfiguration"`
	Error                  string     `json:"error,omitempty"`
	ServiceID              string     `json:"serviceId,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	CompletedAt            *time.Time `json:"completedAt,omitempty"`
}

type Job struct {
	ID           string
	OrgID        string
	OnboardingID string
	Kind         string
	Attempts     int
	MaxAttempts  int
}

const (
	JobStart  = "start"
	JobOpenPR = "open_pr"
)

const MaxRepositoriesPerBatch = 25
