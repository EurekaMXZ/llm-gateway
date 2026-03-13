package domain

import "time"

type PolicyStatus string

const (
	PolicyStatusEnabled  PolicyStatus = "enabled"
	PolicyStatusDisabled PolicyStatus = "disabled"
)

type Policy struct {
	ID               string
	OwnerID          string
	CustomModel      string
	TargetProviderID string
	TargetModel      string
	Priority         int
	ConditionJSON    string
	Status           PolicyStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
