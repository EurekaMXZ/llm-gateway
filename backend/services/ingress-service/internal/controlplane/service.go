package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrMissingBearer = errors.New("missing bearer token")
	ErrMissingAPIKey = errors.New("missing api key")
	ErrUnauthorized  = errors.New("identity unauthorized")
	ErrUpstream      = errors.New("upstream service error")
)

type IdentityUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type APIKeyValidation struct {
	Valid   bool   `json:"valid"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	KeyID   string `json:"key_id"`
	OwnerID string `json:"owner_id"`
}

type Decision struct {
	Allowed  bool             `json:"allowed"`
	Reason   string           `json:"reason"`
	Identity IdentityUser     `json:"identity"`
	APIKey   APIKeyValidation `json:"api_key"`
}

type Service struct {
	identityBaseURL string
	apikeyBaseURL   string
	httpClient      *http.Client
}

func NewService(identityBaseURL string, apikeyBaseURL string, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Service{
		identityBaseURL: strings.TrimRight(strings.TrimSpace(identityBaseURL), "/"),
		apikeyBaseURL:   strings.TrimRight(strings.TrimSpace(apikeyBaseURL), "/"),
		httpClient:      client,
	}
}

func (s *Service) CheckRequest(ctx context.Context, bearerToken string, apiKey string, model string) (Decision, error) {
	bearerToken = strings.TrimSpace(bearerToken)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if bearerToken == "" {
		return Decision{}, ErrMissingBearer
	}
	if apiKey == "" {
		return Decision{}, ErrMissingAPIKey
	}

	identity, err := s.validateIdentity(ctx, bearerToken)
	if err != nil {
		return Decision{}, err
	}

	keyValidation, err := s.validateAPIKey(ctx, apiKey, model)
	if err != nil {
		return Decision{}, err
	}

	decision := Decision{
		Allowed:  false,
		Reason:   keyValidation.Reason,
		Identity: identity,
		APIKey:   keyValidation,
	}
	if !keyValidation.Valid {
		return decision, nil
	}
	if keyValidation.OwnerID != identity.ID {
		decision.Reason = "key_owner_mismatch"
		decision.APIKey.Valid = false
		return decision, nil
	}

	decision.Allowed = true
	decision.Reason = "ok"
	return decision, nil
}

func (s *Service) validateIdentity(ctx context.Context, bearerToken string) (IdentityUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.identityBaseURL+"/v1/auth/validate", nil)
	if err != nil {
		return IdentityUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return IdentityUser{}, fmt.Errorf("%w: identity request failed: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return IdentityUser{}, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return IdentityUser{}, fmt.Errorf("%w: identity status=%d body=%s", ErrUpstream, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		User IdentityUser `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return IdentityUser{}, fmt.Errorf("%w: identity decode failed: %v", ErrUpstream, err)
	}
	if payload.User.ID == "" {
		return IdentityUser{}, fmt.Errorf("%w: identity response missing user id", ErrUpstream)
	}
	return payload.User, nil
}

func (s *Service) validateAPIKey(ctx context.Context, apiKey string, model string) (APIKeyValidation, error) {
	body, err := json.Marshal(map[string]string{
		"api_key": apiKey,
		"model":   model,
	})
	if err != nil {
		return APIKeyValidation{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apikeyBaseURL+"/v1/keys/validate", bytes.NewReader(body))
	if err != nil {
		return APIKeyValidation{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return APIKeyValidation{}, fmt.Errorf("%w: apikey request failed: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return APIKeyValidation{}, fmt.Errorf("%w: apikey status=%d body=%s", ErrUpstream, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload APIKeyValidation
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return APIKeyValidation{}, fmt.Errorf("%w: apikey decode failed: %v", ErrUpstream, err)
	}
	return payload, nil
}

func Is(err error, target error) bool {
	return errors.Is(err, target)
}
