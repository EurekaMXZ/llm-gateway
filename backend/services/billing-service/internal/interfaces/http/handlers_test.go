package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	billingapp "llm-gateway/backend/services/billing-service/internal/app"
)

func TestBillingPriceAndWalletFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := billingapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	setPrice := doJSON(engine, http.MethodPost, "/v1/prices", map[string]any{
		"actor_id":            "u-1",
		"actor_is_superuser":  false,
		"owner_id":            "u-1",
		"provider_id":         "provider-a",
		"model":               "gpt-4o-mini",
		"input_price_per_1k":  0.2,
		"output_price_per_1k": 0.4,
		"currency":            "usd",
	})
	if setPrice.Code != http.StatusOK {
		t.Fatalf("set price status=%d body=%s", setPrice.Code, setPrice.Body.String())
	}

	topUp := doJSON(engine, http.MethodPost, "/v1/wallets/u-1/topup", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"amount_cents":       500,
		"reason":             "seed",
	})
	if topUp.Code != http.StatusOK {
		t.Fatalf("topup status=%d body=%s", topUp.Code, topUp.Body.String())
	}

	deduct := doJSON(engine, http.MethodPost, "/v1/wallets/u-1/deduct", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"amount_cents":       200,
		"reason":             "usage",
	})
	if deduct.Code != http.StatusOK {
		t.Fatalf("deduct status=%d body=%s", deduct.Code, deduct.Body.String())
	}

	wallet := doJSON(engine, http.MethodGet, "/v1/wallets/u-1", nil)
	if wallet.Code != http.StatusOK {
		t.Fatalf("wallet status=%d body=%s", wallet.Code, wallet.Body.String())
	}
}

func TestBillingInsufficientBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := billingapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	deduct := doJSON(engine, http.MethodPost, "/v1/wallets/u-1/deduct", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"amount_cents":       200,
		"reason":             "usage",
	})
	if deduct.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got=%d body=%s", deduct.Code, deduct.Body.String())
	}
}

func doJSON(engine *gin.Engine, method string, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}
