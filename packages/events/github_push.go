package events

import "time"

const GitHubPushReceivedTopic = "github.push"

type GitHubPushReceived struct {
	DeliveryID              string    `json:"delivery_id"`
	Event                   string    `json:"event"`
	RepositoryID            int64     `json:"repository_id"`
	RepositoryFullName      string    `json:"repository_full_name"`
	RepositoryDefaultBranch string    `json:"repository_default_branch"`
	InstallationID          *int64    `json:"installation_id,omitempty"`
	Ref                     string    `json:"ref"`
	BeforeSHA               string    `json:"before_sha"`
	AfterSHA                string    `json:"after_sha"`
	HeadCommitSHA           *string   `json:"head_commit_sha,omitempty"`
	SenderLogin             string    `json:"sender_login"`
	ReceivedAt              time.Time `json:"received_at"`
}
