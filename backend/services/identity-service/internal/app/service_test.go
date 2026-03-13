package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"llm-gateway/backend/services/identity-service/internal/domain"
)

func TestCheckAccessHierarchy(t *testing.T) {
	ctx := context.Background()
	svc := NewService("test-secret")
	super, err := svc.BootstrapSuperuser(ctx, "root", "pass", "Root")
	if err != nil {
		t.Fatalf("bootstrap superuser: %v", err)
	}
	adminA, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   super.ID,
		ActorRole: domain.RoleSuperuser,
		Username:  "admin-a",
		Password:  "pass",
		Role:      domain.RoleAdmin,
		ParentID:  super.ID,
	})
	if err != nil {
		t.Fatalf("create adminA: %v", err)
	}
	adminB, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   super.ID,
		ActorRole: domain.RoleSuperuser,
		Username:  "admin-b",
		Password:  "pass",
		Role:      domain.RoleAdmin,
		ParentID:  super.ID,
	})
	if err != nil {
		t.Fatalf("create adminB: %v", err)
	}
	userA1, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   adminA.ID,
		ActorRole: domain.RoleAdmin,
		Username:  "user-a1",
		Password:  "pass",
		Role:      domain.RoleUser,
		ParentID:  adminA.ID,
	})
	if err != nil {
		t.Fatalf("create userA1: %v", err)
	}

	cases := []struct {
		name    string
		actor   string
		owner   string
		allowed bool
	}{
		{name: "superuser global", actor: super.ID, owner: userA1.ID, allowed: true},
		{name: "admin subtree", actor: adminA.ID, owner: userA1.ID, allowed: true},
		{name: "admin peer subtree denied", actor: adminB.ID, owner: userA1.ID, allowed: false},
		{name: "user self", actor: userA1.ID, owner: userA1.ID, allowed: true},
		{name: "user other denied", actor: userA1.ID, owner: adminA.ID, allowed: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, _, err := svc.CheckAccess(ctx, tc.actor, tc.owner)
			if err != nil {
				t.Fatalf("check access error: %v", err)
			}
			if allowed != tc.allowed {
				t.Fatalf("allowed=%v want=%v", allowed, tc.allowed)
			}
		})
	}
}

func TestTokenIssueAndValidation(t *testing.T) {
	ctx := context.Background()
	svc := NewService("test-secret")
	_, err := svc.BootstrapSuperuser(ctx, "root", "pass", "Root")
	if err != nil {
		t.Fatalf("bootstrap superuser: %v", err)
	}
	token, user, _, err := svc.Authenticate(ctx, "root", "pass")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if token == "" || user.ID == "" {
		t.Fatalf("invalid token response")
	}

	validated, err := svc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if validated.Username != "root" {
		t.Fatalf("validated user mismatch: %s", validated.Username)
	}
}

func TestCreateUserPermissionScope(t *testing.T) {
	ctx := context.Background()
	svc := NewService("test-secret")

	super, err := svc.BootstrapSuperuser(ctx, "root", "pass", "Root")
	if err != nil {
		t.Fatalf("bootstrap superuser: %v", err)
	}
	adminA, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   super.ID,
		ActorRole: domain.RoleSuperuser,
		Username:  "admin-a",
		Password:  "pass",
		Role:      domain.RoleAdmin,
		ParentID:  super.ID,
	})
	if err != nil {
		t.Fatalf("create admin-a: %v", err)
	}
	adminB, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   super.ID,
		ActorRole: domain.RoleSuperuser,
		Username:  "admin-b",
		Password:  "pass",
		Role:      domain.RoleAdmin,
		ParentID:  super.ID,
	})
	if err != nil {
		t.Fatalf("create admin-b: %v", err)
	}

	_, err = svc.CreateUser(ctx, CreateUserInput{
		ActorID:   adminA.ID,
		ActorRole: domain.RoleAdmin,
		Username:  "user-outside",
		Password:  "pass",
		Role:      domain.RoleUser,
		ParentID:  adminB.ID,
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden outside subtree, got %v", err)
	}

	child, err := svc.CreateUser(ctx, CreateUserInput{
		ActorID:   adminA.ID,
		ActorRole: domain.RoleAdmin,
		Username:  "user-inside",
		Password:  "pass",
		Role:      domain.RoleUser,
	})
	if err != nil {
		t.Fatalf("create subtree user: %v", err)
	}
	if child.ParentID != adminA.ID {
		t.Fatalf("expected default parent=%s got=%s", adminA.ID, child.ParentID)
	}
}

func TestBootstrapSuperuserAtomicUniqueness(t *testing.T) {
	ctx := context.Background()
	svc := NewService("test-secret")

	var (
		successes int32
		conflicts int32
		wg        sync.WaitGroup
	)

	usernames := []string{"root-a", "root-b"}
	for _, username := range usernames {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			_, err := svc.BootstrapSuperuser(ctx, u, "pass", "Root")
			switch {
			case err == nil:
				atomic.AddInt32(&successes, 1)
			case IsDomainError(err, domain.ErrSuperuserAlreadyExists):
				atomic.AddInt32(&conflicts, 1)
			default:
				t.Errorf("unexpected bootstrap error: %v", err)
			}
		}(username)
	}
	wg.Wait()

	if successes != 1 || conflicts != 1 {
		t.Fatalf("unexpected bootstrap outcome: successes=%d conflicts=%d", successes, conflicts)
	}
}
