package app

import (
	"context"
	"testing"

	"llm-gateway/backend/services/billing-service/internal/domain"
)

func TestSetPriceOwnershipConstraint(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.SetPrice(ctx, SetPriceInput{
		ActorID:          "u-a",
		ActorIsSuperuser: false,
		OwnerID:          "u-b",
		ProviderID:       "provider-a",
		Model:            "gpt-4o-mini",
		InputPricePer1K:  0.2,
		OutputPricePer1K: 0.4,
		Currency:         "USD",
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestWalletTopUpDeductAndInsufficientBalance(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	wallet, _, err := svc.TopUp(ctx, WalletOperationInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		AmountCents:      1000,
		Reason:           "initial",
	})
	if err != nil {
		t.Fatalf("topup failed: %v", err)
	}
	if wallet.BalanceCents != 1000 {
		t.Fatalf("expected 1000 cents, got %d", wallet.BalanceCents)
	}

	wallet, _, err = svc.Deduct(ctx, WalletOperationInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		AmountCents:      300,
		Reason:           "usage",
	})
	if err != nil {
		t.Fatalf("deduct failed: %v", err)
	}
	if wallet.BalanceCents != 700 {
		t.Fatalf("expected 700 cents, got %d", wallet.BalanceCents)
	}

	_, _, err = svc.Deduct(ctx, WalletOperationInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		AmountCents:      1000,
		Reason:           "overspend",
	})
	if !IsDomainError(err, domain.ErrInsufficientBalance) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
}

func TestActorCanWriteAllowsSubtreeWalletWrite(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	wallet, _, err := svc.TopUp(ctx, WalletOperationInput{
		ActorID:          "admin-a",
		ActorIsSuperuser: false,
		ActorCanWrite:    true,
		OwnerID:          "u-subtree",
		AmountCents:      500,
		Reason:           "subtree-topup",
	})
	if err != nil {
		t.Fatalf("expected actor_can_write topup success, got %v", err)
	}
	if wallet.BalanceCents != 500 {
		t.Fatalf("expected balance=500, got %d", wallet.BalanceCents)
	}
}
