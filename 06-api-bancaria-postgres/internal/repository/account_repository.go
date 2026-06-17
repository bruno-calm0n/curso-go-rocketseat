package repository

import (
	"context"

	"api-bancaria-postgres/internal/domain"
)

// AccountRepository define as operacoes bancarias usadas pelos handlers.
type AccountRepository interface {
	Create(ctx context.Context, input domain.CreateAccountInput) (domain.Account, error)
	GetBalance(ctx context.Context, ref domain.AccountRef) (domain.Balance, error)
	Deposit(ctx context.Context, ref domain.AccountRef, amount float64) (domain.Balance, error)
	Withdraw(ctx context.Context, ref domain.AccountRef, amount float64) (domain.Balance, error)
	Transfer(ctx context.Context, from domain.AccountRef, to domain.AccountRef, amount float64) (domain.TransferResult, error)
	Close(ctx context.Context, ref domain.AccountRef) (domain.Account, error)
}
