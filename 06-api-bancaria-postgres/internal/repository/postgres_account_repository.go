package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"api-bancaria-postgres/internal/domain"
)

type PostgresAccountRepository struct {
	db *sql.DB
}

// NewPostgresAccountRepository recebe a conexao pronta por injecao de dependencia.
func NewPostgresAccountRepository(db *sql.DB) *PostgresAccountRepository {
	return &PostgresAccountRepository{db: db}
}

// Create escolhe a tabela correta conforme o tipo da conta.
func (r *PostgresAccountRepository) Create(ctx context.Context, input domain.CreateAccountInput) (domain.Account, error) {
	switch input.Type {
	case domain.AccountTypePF:
		return r.createPF(ctx, input)
	case domain.AccountTypePJ:
		return r.createPJ(ctx, input)
	default:
		return domain.Account{}, domain.ErrInvalidAccountType
	}
}

// GetBalance consulta somente o saldo da conta solicitada.
func (r *PostgresAccountRepository) GetBalance(ctx context.Context, ref domain.AccountRef) (domain.Balance, error) {
	query, err := balanceQuery(ref.Type, false)
	if err != nil {
		return domain.Balance{}, err
	}

	var balance float64
	err = r.db.QueryRowContext(ctx, query, ref.ID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Balance{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Balance{}, err
	}

	return domain.Balance{
		ID:               domain.FormatAccountID(ref.Type, ref.ID),
		AvailableBalance: balance,
	}, nil
}

// Deposit atualiza o saldo com uma soma atomica no banco.
func (r *PostgresAccountRepository) Deposit(ctx context.Context, ref domain.AccountRef, amount float64) (domain.Balance, error) {
	table, err := tableName(ref.Type)
	if err != nil {
		return domain.Balance{}, err
	}

	query := fmt.Sprintf("UPDATE %s SET saldo = saldo + $1 WHERE id = $2 RETURNING saldo", table)

	var balance float64
	err = r.db.QueryRowContext(ctx, query, amount, ref.ID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Balance{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Balance{}, err
	}

	return domain.Balance{ID: domain.FormatAccountID(ref.Type, ref.ID), AvailableBalance: balance}, nil
}

// Withdraw usa transacao para bloquear a linha e evitar saldo negativo.
func (r *PostgresAccountRepository) Withdraw(ctx context.Context, ref domain.AccountRef, amount float64) (domain.Balance, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Balance{}, err
	}
	defer tx.Rollback()

	currentBalance, err := balanceForUpdate(ctx, tx, ref)
	if err != nil {
		return domain.Balance{}, err
	}
	if currentBalance < amount {
		return domain.Balance{}, domain.ErrInsufficientFunds
	}

	balance, err := updateBalance(ctx, tx, ref, -amount)
	if err != nil {
		return domain.Balance{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Balance{}, err
	}

	return balance, nil
}

// Transfer bloqueia as contas envolvidas e move o saldo na mesma transacao.
func (r *PostgresAccountRepository) Transfer(ctx context.Context, from domain.AccountRef, to domain.AccountRef, amount float64) (domain.TransferResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.TransferResult{}, err
	}
	defer tx.Rollback()

	fromBalance, err := balanceForUpdate(ctx, tx, from)
	if err != nil {
		return domain.TransferResult{}, err
	}
	if fromBalance < amount {
		return domain.TransferResult{}, domain.ErrInsufficientFunds
	}

	if _, err := balanceForUpdate(ctx, tx, to); err != nil {
		return domain.TransferResult{}, err
	}

	updatedFrom, err := updateBalance(ctx, tx, from, -amount)
	if err != nil {
		return domain.TransferResult{}, err
	}

	updatedTo, err := updateBalance(ctx, tx, to, amount)
	if err != nil {
		return domain.TransferResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.TransferResult{}, err
	}

	return domain.TransferResult{From: updatedFrom, To: updatedTo, Amount: amount}, nil
}

// Close busca a conta antes de deletar para retornar os dados removidos.
func (r *PostgresAccountRepository) Close(ctx context.Context, ref domain.AccountRef) (domain.Account, error) {
	account, err := r.findAccount(ctx, ref)
	if err != nil {
		return domain.Account{}, err
	}

	table, err := tableName(ref.Type)
	if err != nil {
		return domain.Account{}, err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", table)
	if _, err := r.db.ExecContext(ctx, query, ref.ID); err != nil {
		return domain.Account{}, err
	}

	return account, nil
}

// createPF insere uma conta de pessoa fisica.
func (r *PostgresAccountRepository) createPF(ctx context.Context, input domain.CreateAccountInput) (domain.Account, error) {
	query := `
		INSERT INTO pessoa_fisica
			(renda_mensal, idade, nome_completo, celular, email, categoria, saldo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id int64
	err := r.db.QueryRowContext(
		ctx,
		query,
		input.Income,
		input.Age,
		input.FullName,
		input.Phone,
		input.Email,
		input.Category,
		input.AvailableBalance,
	).Scan(&id)
	if err != nil {
		return domain.Account{}, err
	}

	return domain.Account{
		ID:               domain.FormatAccountID(domain.AccountTypePF, id),
		Type:             domain.AccountTypePF,
		Income:           input.Income,
		Age:              input.Age,
		FullName:         input.FullName,
		Phone:            input.Phone,
		Email:            input.Email,
		Category:         input.Category,
		AvailableBalance: input.AvailableBalance,
	}, nil
}

// createPJ insere uma conta de pessoa juridica.
func (r *PostgresAccountRepository) createPJ(ctx context.Context, input domain.CreateAccountInput) (domain.Account, error) {
	query := `
		INSERT INTO pessoa_juridica
			(faturamento, idade, nome_fantasia, celular, email_corporativo, categoria, saldo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id int64
	err := r.db.QueryRowContext(
		ctx,
		query,
		input.Revenue,
		input.Age,
		input.TradeName,
		input.Phone,
		input.CorporateEmail,
		input.Category,
		input.AvailableBalance,
	).Scan(&id)
	if err != nil {
		return domain.Account{}, err
	}

	return domain.Account{
		ID:               domain.FormatAccountID(domain.AccountTypePJ, id),
		Type:             domain.AccountTypePJ,
		Revenue:          input.Revenue,
		Age:              input.Age,
		TradeName:        input.TradeName,
		Phone:            input.Phone,
		CorporateEmail:   input.CorporateEmail,
		Category:         input.Category,
		AvailableBalance: input.AvailableBalance,
	}, nil
}

// findAccount direciona a busca para a tabela correta.
func (r *PostgresAccountRepository) findAccount(ctx context.Context, ref domain.AccountRef) (domain.Account, error) {
	switch ref.Type {
	case domain.AccountTypePF:
		return r.findPF(ctx, ref.ID)
	case domain.AccountTypePJ:
		return r.findPJ(ctx, ref.ID)
	default:
		return domain.Account{}, domain.ErrInvalidAccountType
	}
}

// findPF monta a resposta de uma conta pessoa fisica.
func (r *PostgresAccountRepository) findPF(ctx context.Context, id int64) (domain.Account, error) {
	var account domain.Account
	account.Type = domain.AccountTypePF
	account.ID = domain.FormatAccountID(domain.AccountTypePF, id)

	err := r.db.QueryRowContext(ctx, `
		SELECT renda_mensal, idade, nome_completo, celular, email, categoria, saldo
		FROM pessoa_fisica
		WHERE id = $1
	`, id).Scan(
		&account.Income,
		&account.Age,
		&account.FullName,
		&account.Phone,
		&account.Email,
		&account.Category,
		&account.AvailableBalance,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}

	return account, nil
}

// findPJ monta a resposta de uma conta pessoa juridica.
func (r *PostgresAccountRepository) findPJ(ctx context.Context, id int64) (domain.Account, error) {
	var account domain.Account
	account.Type = domain.AccountTypePJ
	account.ID = domain.FormatAccountID(domain.AccountTypePJ, id)

	err := r.db.QueryRowContext(ctx, `
		SELECT faturamento, idade, nome_fantasia, celular, email_corporativo, categoria, saldo
		FROM pessoa_juridica
		WHERE id = $1
	`, id).Scan(
		&account.Revenue,
		&account.Age,
		&account.TradeName,
		&account.Phone,
		&account.CorporateEmail,
		&account.Category,
		&account.AvailableBalance,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, err
	}

	return account, nil
}

// balanceForUpdate le o saldo com bloqueio de linha dentro da transacao.
func balanceForUpdate(ctx context.Context, tx *sql.Tx, ref domain.AccountRef) (float64, error) {
	query, err := balanceQuery(ref.Type, true)
	if err != nil {
		return 0, err
	}

	var balance float64
	err = tx.QueryRowContext(ctx, query, ref.ID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrAccountNotFound
	}
	if err != nil {
		return 0, err
	}

	return balance, nil
}

// updateBalance aplica a variacao positiva ou negativa e retorna o saldo novo.
func updateBalance(ctx context.Context, tx *sql.Tx, ref domain.AccountRef, amount float64) (domain.Balance, error) {
	table, err := tableName(ref.Type)
	if err != nil {
		return domain.Balance{}, err
	}

	query := fmt.Sprintf("UPDATE %s SET saldo = saldo + $1 WHERE id = $2 RETURNING saldo", table)

	var balance float64
	err = tx.QueryRowContext(ctx, query, amount, ref.ID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Balance{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Balance{}, err
	}

	return domain.Balance{ID: domain.FormatAccountID(ref.Type, ref.ID), AvailableBalance: balance}, nil
}

// balanceQuery monta a query usando apenas nomes de tabela permitidos.
func balanceQuery(accountType domain.AccountType, lock bool) (string, error) {
	table, err := tableName(accountType)
	if err != nil {
		return "", err
	}

	query := fmt.Sprintf("SELECT saldo FROM %s WHERE id = $1", table)
	if lock {
		query += " FOR UPDATE"
	}

	return query, nil
}

// tableName evita concatenar entrada do usuario diretamente no SQL.
func tableName(accountType domain.AccountType) (string, error) {
	switch accountType {
	case domain.AccountTypePF:
		return "pessoa_fisica", nil
	case domain.AccountTypePJ:
		return "pessoa_juridica", nil
	default:
		return "", domain.ErrInvalidAccountType
	}
}
