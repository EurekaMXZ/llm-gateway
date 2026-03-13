package domain

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrForbidden         = errors.New("forbidden")
	ErrTemplateNotFound  = errors.New("template not found")
	ErrTemplateNameTaken = errors.New("template scene already exists for owner")
	ErrTemplateDisabled  = errors.New("template disabled")
	ErrRenderValidation  = errors.New("render validation failed")
)
