package domain

import "errors"

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrForbidden           = errors.New("forbidden")
	ErrPriceNotFound       = errors.New("price not found")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
)
