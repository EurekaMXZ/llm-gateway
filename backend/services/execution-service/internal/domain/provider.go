package domain

import "time"

type ProviderStatus string

const (
	ProviderStatusEnabled  ProviderStatus = "enabled"
	ProviderStatusDisabled ProviderStatus = "disabled"
)

type Provider struct {
	ID        string
	OwnerID   string
	Name      string
	Protocol  string
	BaseURL   string
	APIKey    string
	Status    ProviderStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
