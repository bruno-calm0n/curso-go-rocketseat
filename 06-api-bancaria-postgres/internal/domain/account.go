package domain

import "errors"

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrInvalidAccountID   = errors.New("invalid account id")
	ErrInvalidAccountType = errors.New("invalid account type")
)

type AccountType string

const (
	// AccountTypePF representa conta de pessoa fisica.
	AccountTypePF AccountType = "pf"
	// AccountTypePJ representa conta de pessoa juridica.
	AccountTypePJ AccountType = "pj"
)

// AccountRef guarda o tipo e o id numerico usados nas tabelas.
type AccountRef struct {
	Type AccountType
	ID   int64
}

// Account representa a resposta publica de uma conta bancaria.
type Account struct {
	ID               string      `json:"id"`
	Type             AccountType `json:"tipo"`
	Income           float64     `json:"renda_mensal,omitempty"`
	Revenue          float64     `json:"faturamento,omitempty"`
	Age              int         `json:"idade"`
	FullName         string      `json:"nome_completo,omitempty"`
	TradeName        string      `json:"nome_fantasia,omitempty"`
	Phone            string      `json:"celular"`
	Email            string      `json:"email,omitempty"`
	CorporateEmail   string      `json:"email_corporativo,omitempty"`
	Category         string      `json:"categoria"`
	AvailableBalance float64     `json:"saldo"`
}

// CreateAccountInput carrega os dados validados antes de salvar a conta.
type CreateAccountInput struct {
	Type             AccountType
	Income           float64
	Revenue          float64
	Age              int
	FullName         string
	TradeName        string
	Phone            string
	Email            string
	CorporateEmail   string
	Category         string
	AvailableBalance float64
}

// Balance representa a resposta das operacoes que mudam saldo.
type Balance struct {
	ID               string  `json:"id"`
	AvailableBalance float64 `json:"saldo"`
}

// TransferResult retorna os dois saldos atualizados apos a transferencia.
type TransferResult struct {
	From   Balance `json:"origem"`
	To     Balance `json:"destino"`
	Amount float64 `json:"valor"`
}
