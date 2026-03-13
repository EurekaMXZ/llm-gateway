package app

import (
	"context"
	"testing"
	"time"
)

func TestKeyLifecycleAndValidation(t *testing.T) {
	ctx := context.Background()
	svc := NewService()
	record, plainKey, err := svc.CreateKey(ctx, CreateKeyInput{
		OwnerID:       "u-1",
		Name:          "dev-key",
		AllowedModels: []string{"gpt-4o-mini", "gpt-4.1-mini"},
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if plainKey == "" || record.ID == "" {
		t.Fatalf("invalid key create response")
	}

	res, err := svc.Validate(ctx, plainKey, "gpt-4o-mini", time.Now().UTC())
	if err != nil {
		t.Fatalf("validate key: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid key, got reason=%s", res.Reason)
	}

	res, err = svc.Validate(ctx, plainKey, "not-allowed-model", time.Now().UTC())
	if err != nil {
		t.Fatalf("validate forbidden model: %v", err)
	}
	if res.Valid || res.Reason != "model_forbidden" {
		t.Fatalf("expected model forbidden, got %+v", res)
	}

	res, err = svc.Validate(ctx, plainKey, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("validate missing model: %v", err)
	}
	if res.Valid || res.Reason != "model_required" {
		t.Fatalf("expected model required, got %+v", res)
	}

	if _, err = svc.DisableKey(ctx, record.ID); err != nil {
		t.Fatalf("disable key: %v", err)
	}
	res, err = svc.Validate(ctx, plainKey, "gpt-4o-mini", time.Now().UTC())
	if err != nil {
		t.Fatalf("validate disabled key: %v", err)
	}
	if res.Valid || res.Reason != "key_disabled" {
		t.Fatalf("expected key disabled, got %+v", res)
	}

	if _, err = svc.EnableKey(ctx, record.ID); err != nil {
		t.Fatalf("enable key: %v", err)
	}
	res, err = svc.Validate(ctx, plainKey, "gpt-4o-mini", time.Now().UTC())
	if err != nil {
		t.Fatalf("validate reenabled key: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected key re-enabled, got %+v", res)
	}
}

func TestExpiredKey(t *testing.T) {
	ctx := context.Background()
	svc := NewService()
	expiredAt := time.Now().UTC().Add(-time.Hour)
	_, _, err := svc.CreateKey(ctx, CreateKeyInput{
		OwnerID:   "u-1",
		Name:      "expired-invalid",
		ExpiresAt: &expiredAt,
	})
	if err == nil {
		t.Fatalf("expected invalid input for already-expired key")
	}

	future := time.Now().UTC().Add(time.Minute)
	record, plainKey, err := svc.CreateKey(ctx, CreateKeyInput{
		OwnerID:   "u-1",
		Name:      "expiring",
		ExpiresAt: &future,
	})
	if err != nil {
		t.Fatalf("create expiring key: %v", err)
	}

	res, err := svc.Validate(ctx, plainKey, "", time.Now().UTC().Add(2*time.Minute))
	if err != nil {
		t.Fatalf("validate expiring key: %v", err)
	}
	if res.Valid || res.Reason != "key_expired" || res.KeyID != record.ID {
		t.Fatalf("expected expired key, got %+v", res)
	}
}
