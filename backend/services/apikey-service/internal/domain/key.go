package domain

import "time"

type KeyStatus string

const (
	KeyStatusEnabled  KeyStatus = "enabled"
	KeyStatusDisabled KeyStatus = "disabled"
)

type APIKey struct {
	ID            string
	OwnerID       string
	Name          string
	SecretHash    string
	AllowedModels []string
	Status        KeyStatus
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
