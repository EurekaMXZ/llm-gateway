package domain

import "time"

type Price struct {
	ID               string
	OwnerID          string
	ProviderID       string
	Model            string
	InputPricePer1K  float64
	OutputPricePer1K float64
	Currency         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
