package domain

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrKeyNotFound    = errors.New("key not found")
	ErrKeyExpired     = errors.New("key expired")
	ErrKeyDisabled    = errors.New("key disabled")
	ErrModelForbidden = errors.New("model not allowed")
)
