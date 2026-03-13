package domain

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrForbidden         = errors.New("forbidden")
	ErrProviderNotFound  = errors.New("provider not found")
	ErrProviderNameTaken = errors.New("provider name already exists")
	ErrProviderDisabled  = errors.New("provider is disabled")
	ErrModelNotFound     = errors.New("model not found")
	ErrModelNameTaken    = errors.New("model name already exists for provider")
)
