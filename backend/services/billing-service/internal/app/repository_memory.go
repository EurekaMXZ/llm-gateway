package app

import (
	"context"
	"sort"
	"sync"
	"time"

	"llm-gateway/backend/services/billing-service/internal/domain"
)

type InMemoryRepository struct {
	mu           sync.RWMutex
	prices       map[string]domain.Price
	priceByKey   map[string]string
	wallets      map[string]domain.Wallet
	transactions map[string][]domain.WalletTransaction
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		prices:       map[string]domain.Price{},
		priceByKey:   map[string]string{},
		wallets:      map[string]domain.Wallet{},
		transactions: map[string][]domain.WalletTransaction{},
	}
}

func (r *InMemoryRepository) UpsertPrice(_ context.Context, price domain.Price) (domain.Price, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := priceKey(price.OwnerID, price.ProviderID, price.Model, price.Currency)
	if id, exists := r.priceByKey[key]; exists {
		existing := r.prices[id]
		existing.InputPricePer1K = price.InputPricePer1K
		existing.OutputPricePer1K = price.OutputPricePer1K
		existing.UpdatedAt = price.UpdatedAt
		r.prices[id] = existing
		return existing, nil
	}
	r.prices[price.ID] = price
	r.priceByKey[key] = price.ID
	return price, nil
}

func (r *InMemoryRepository) GetPriceByID(_ context.Context, id string) (domain.Price, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	price, ok := r.prices[id]
	if !ok {
		return domain.Price{}, domain.ErrPriceNotFound
	}
	return price, nil
}

func (r *InMemoryRepository) ListPrices(_ context.Context, ownerID string) ([]domain.Price, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Price, 0)
	for _, price := range r.prices {
		if ownerID != "" && price.OwnerID != ownerID {
			continue
		}
		list = append(list, price)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list, nil
}

func (r *InMemoryRepository) GetWallet(_ context.Context, ownerID string) (domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wallet, ok := r.wallets[ownerID]
	if !ok {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	return wallet, nil
}

func (r *InMemoryRepository) ApplyWalletDelta(_ context.Context, ownerID string, deltaCents int64, txID string, txType domain.TransactionType, reason string, now time.Time) (domain.Wallet, domain.WalletTransaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wallet, ok := r.wallets[ownerID]
	if !ok {
		wallet = domain.Wallet{OwnerID: ownerID, BalanceCents: 0, UpdatedAt: now}
	}
	next := wallet.BalanceCents + deltaCents
	if next < 0 {
		return domain.Wallet{}, domain.WalletTransaction{}, domain.ErrInsufficientBalance
	}
	wallet.BalanceCents = next
	wallet.UpdatedAt = now
	r.wallets[ownerID] = wallet

	tx := domain.WalletTransaction{
		ID:                txID,
		OwnerID:           ownerID,
		Type:              txType,
		AmountCents:       deltaCents,
		BalanceAfterCents: next,
		Reason:            reason,
		CreatedAt:         now,
	}
	r.transactions[ownerID] = append(r.transactions[ownerID], tx)
	return wallet, tx, nil
}

func (r *InMemoryRepository) ListTransactions(_ context.Context, ownerID string, limit int) ([]domain.WalletTransaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	txs := append([]domain.WalletTransaction(nil), r.transactions[ownerID]...)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].CreatedAt.After(txs[j].CreatedAt)
	})
	if limit > 0 && len(txs) > limit {
		txs = txs[:limit]
	}
	return txs, nil
}

func priceKey(ownerID string, providerID string, model string, currency string) string {
	return ownerID + "::" + providerID + "::" + model + "::" + currency
}
