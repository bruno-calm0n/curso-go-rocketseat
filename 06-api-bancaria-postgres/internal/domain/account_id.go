package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAccountID converte ids publicos como pf_1 e pj_1 para referencia interna.
func ParseAccountID(value string) (AccountRef, error) {
	parts := strings.Split(value, "_")
	if len(parts) != 2 {
		return AccountRef{}, ErrInvalidAccountID
	}

	accountType := AccountType(strings.ToLower(parts[0]))
	if accountType != AccountTypePF && accountType != AccountTypePJ {
		return AccountRef{}, ErrInvalidAccountType
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return AccountRef{}, ErrInvalidAccountID
	}

	return AccountRef{Type: accountType, ID: id}, nil
}

// FormatAccountID cria o id publico usado nas rotas da API.
func FormatAccountID(accountType AccountType, id int64) string {
	return fmt.Sprintf("%s_%d", accountType, id)
}
