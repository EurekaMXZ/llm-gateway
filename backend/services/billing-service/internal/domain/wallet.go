package domain

import "time"

type Wallet struct {
	OwnerID      string
	BalanceCents int64
	UpdatedAt    time.Time
}

type TransactionType string

const (
	TransactionTypeTopUp  TransactionType = "topup"
	TransactionTypeDeduct TransactionType = "deduct"
)

type WalletTransaction struct {
	ID                string
	OwnerID           string
	Type              TransactionType
	AmountCents       int64
	BalanceAfterCents int64
	Reason            string
	CreatedAt         time.Time
}
