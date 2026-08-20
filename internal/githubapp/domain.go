package githubapp

import (
	"fmt"
	"time"
)

type State string

const (
	StateSelected   State = "selected"
	StateRunQueued  State = "run_queued"
	StateRunRunning State = "run_running"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
)

func (s State) Validate() error {
	switch s {
	case StateSelected, StateRunQueued, StateRunRunning, StateCompleted, StateFailed:
		return nil
	default:
		return fmt.Errorf("invalid repository import state %q", s)
	}
}

type Installation struct {
	OrgID                string    `json:"-"`
	GitHubInstallationID int64     `json:"installationId"`
	CreatedAt            time.Time `json:"createdAt"`
	AccountID            int64     `json:"accountId,omitempty"`
	AccountLogin         string    `json:"account,omitempty"`
	AccountType          string    `json:"accountType,omitempty"`
	Suspended            bool      `json:"suspended"`
}

type Repository struct {
	GitHubID      int64  `json:"githubId"`
	OwnerID       int64  `json:"ownerId"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	URL           string `json:"url"`
	DefaultBranch string `json:"defaultBranch"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
}

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
	ID             string     `json:"id"`
	OrgID          string     `json:"-"`
	GitHubOwnerID  int64      `json:"githubOwnerId"`
	GitHubRepo     string     `json:"githubRepo"`
	TeamID         string     `json:"teamId"`
	TeamName       string     `json:"team"`
	Status         State      `json:"status"`
	Steps          []Step     `json:"steps"`
	Branch         string     `json:"branch"`
	RunID          int64      `json:"-"`
	RunURL         string     `json:"runUrl,omitempty"`
	PRURL          string     `json:"prUrl,omitempty"`
	Error          string     `json:"error,omitempty"`
	ServiceID      string     `json:"serviceId,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	RunStartedAt   *time.Time `json:"runStartedAt,omitempty"`
	RunCompletedAt *time.Time `json:"runCompletedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
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
