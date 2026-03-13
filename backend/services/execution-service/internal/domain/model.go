package domain

import "time"

type ModelStatus string

const (
	ModelStatusEnabled  ModelStatus = "enabled"
	ModelStatusDisabled ModelStatus = "disabled"
)

type Model struct {
	ID            string
	ProviderID    string
	OwnerID       string
	Name          string
	UpstreamModel string
	Status        ModelStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
