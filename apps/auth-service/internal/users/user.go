package users

import "time"

type User struct {
	ID        int64     `json:"id"`
	GitHubID  int64     `json:"github_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
