package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	billingapp "llm-gateway/backend/services/billing-service/internal/app"
	"llm-gateway/backend/services/billing-service/internal/domain"
)

type Handler struct {
	service *billingapp.Service
}

func NewHandler(service *billingapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	v1 := engine.Group("/v1")
	{
		v1.POST("/prices", h.setPrice)
		v1.GET("/prices", h.listPrices)
		v1.GET("/prices/:id", h.getPrice)

		v1.GET("/wallets/:owner_id", h.getWallet)
		v1.POST("/wallets/:owner_id/topup", h.topUp)
		v1.POST("/wallets/:owner_id/deduct", h.deduct)
		v1.GET("/wallets/:owner_id/transactions", h.listTransactions)
	}
}

type setPriceRequest struct {
	ActorID          string  `json:"actor_id"`
	ActorIsSuperuser bool    `json:"actor_is_superuser"`
	ActorCanWrite    bool    `json:"actor_can_write"`
	OwnerID          string  `json:"owner_id"`
	ProviderID       string  `json:"provider_id"`
	Model            string  `json:"model"`
	InputPricePer1K  float64 `json:"input_price_per_1k"`
	OutputPricePer1K float64 `json:"output_price_per_1k"`
	Currency         string  `json:"currency"`
}

func (h *Handler) setPrice(c *gin.Context) {
	var req setPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "billing.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	price, err := h.service.SetPrice(c.Request.Context(), billingapp.SetPriceInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          req.OwnerID,
		ProviderID:       req.ProviderID,
		Model:            req.Model,
		InputPricePer1K:  req.InputPricePer1K,
		OutputPricePer1K: req.OutputPricePer1K,
		Currency:         req.Currency,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"price": priceResponse(price)})
}

func (h *Handler) getPrice(c *gin.Context) {
	price, err := h.service.GetPrice(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"price": priceResponse(price)})
}

func (h *Handler) listPrices(c *gin.Context) {
	prices, err := h.service.ListPrices(c.Request.Context(), c.Query("owner_id"))
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "billing.internal.list_prices_failed", "failed to list prices", "internal")
		return
	}
	items := make([]gin.H, 0, len(prices))
	for _, price := range prices {
		items = append(items, priceResponse(price))
	}
	c.JSON(http.StatusOK, gin.H{"prices": items})
}

func (h *Handler) getWallet(c *gin.Context) {
	wallet, err := h.service.GetWallet(c.Request.Context(), c.Param("owner_id"))
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallet": walletResponse(wallet)})
}

type walletOperationRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
	AmountCents      int64  `json:"amount_cents"`
	Reason           string `json:"reason"`
}

func (h *Handler) topUp(c *gin.Context) {
	h.walletOp(c, true)
}

func (h *Handler) deduct(c *gin.Context) {
	h.walletOp(c, false)
}

func (h *Handler) walletOp(c *gin.Context, isTopUp bool) {
	var req walletOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "billing.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	input := billingapp.WalletOperationInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          c.Param("owner_id"),
		AmountCents:      req.AmountCents,
		Reason:           req.Reason,
	}
	var (
		wallet domain.Wallet
		txn    domain.WalletTransaction
		err    error
	)
	if isTopUp {
		wallet, txn, err = h.service.TopUp(c.Request.Context(), input)
	} else {
		wallet, txn, err = h.service.Deduct(c.Request.Context(), input)
	}
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallet": walletResponse(wallet), "transaction": transactionResponse(txn)})
}

func (h *Handler) listTransactions(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	txs, err := h.service.ListTransactions(c.Request.Context(), c.Param("owner_id"), limit)
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	items := make([]gin.H, 0, len(txs))
	for _, tx := range txs {
		items = append(items, transactionResponse(tx))
	}
	c.JSON(http.StatusOK, gin.H{"transactions": items})
}

func (h *Handler) writeDomainError(c *gin.Context, err error) {
	switch {
	case billingapp.IsDomainError(err, domain.ErrInvalidInput):
		ginx.JSONError(c, http.StatusBadRequest, "billing.validation.invalid_input", err.Error(), "validation")
	case billingapp.IsDomainError(err, domain.ErrForbidden):
		ginx.JSONError(c, http.StatusForbidden, "billing.auth.forbidden", err.Error(), "auth")
	case billingapp.IsDomainError(err, domain.ErrPriceNotFound), billingapp.IsDomainError(err, domain.ErrWalletNotFound):
		ginx.JSONError(c, http.StatusNotFound, "billing.not_found.resource", err.Error(), "not_found")
	case billingapp.IsDomainError(err, domain.ErrInsufficientBalance):
		ginx.JSONError(c, http.StatusBadRequest, "billing.validation.insufficient_balance", err.Error(), "validation")
	default:
		ginx.JSONError(c, http.StatusInternalServerError, "billing.internal.unexpected_error", "unexpected billing error", "internal")
	}
}

func priceResponse(price domain.Price) gin.H {
	return gin.H{
		"id":                  price.ID,
		"owner_id":            price.OwnerID,
		"provider_id":         price.ProviderID,
		"model":               price.Model,
		"input_price_per_1k":  price.InputPricePer1K,
		"output_price_per_1k": price.OutputPricePer1K,
		"currency":            price.Currency,
		"created_at":          price.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":          price.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func walletResponse(wallet domain.Wallet) gin.H {
	return gin.H{
		"owner_id":      wallet.OwnerID,
		"balance_cents": wallet.BalanceCents,
		"updated_at":    wallet.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func transactionResponse(tx domain.WalletTransaction) gin.H {
	return gin.H{
		"id":                  tx.ID,
		"owner_id":            tx.OwnerID,
		"type":                tx.Type,
		"amount_cents":        tx.AmountCents,
		"balance_after_cents": tx.BalanceAfterCents,
		"reason":              tx.Reason,
		"created_at":          tx.CreatedAt.UTC().Format(time.RFC3339),
	}
}
