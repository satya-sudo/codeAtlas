package installations

import "time"

type Installation struct {
	ID                int64     `json:"id"`
	InstallationID    int64     `json:"installation_id"`
	InstalledByUserID *int64    `json:"installed_by_user_id,omitempty"`
	AccountLogin      *string   `json:"account_login,omitempty"`
	AccountType       *string   `json:"account_type,omitempty"`
	SetupAction       *string   `json:"setup_action,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
