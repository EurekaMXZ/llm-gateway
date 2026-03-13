package domain

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrForbidden       = errors.New("forbidden")
	ErrPolicyNotFound  = errors.New("policy not found")
	ErrPolicyNameTaken = errors.New("policy already exists for custom model and priority")
	ErrNoPolicyMatched = errors.New("no policy matched")
)
