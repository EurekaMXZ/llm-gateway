package domain

import "errors"

var (
	ErrInvalidRole            = errors.New("invalid role")
	ErrSuperuserAlreadyExists = errors.New("superuser already exists")
	ErrUsernameTaken          = errors.New("username already exists")
	ErrUserNotFound           = errors.New("user not found")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidToken           = errors.New("invalid token")
	ErrForbidden              = errors.New("forbidden")
)
