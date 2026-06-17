package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var rates = map[string]float64{
	"USD": 0.151,
	"EUR": 0.137,
	"JPY": 16.29,
	"GBP": 0.13,
	"CHF": 0.1402,
	"AUD": 0.2712,
}

func main() {
	// Valida a quantidade exata de argumentos esperados pela CLI.
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "uso: ./convert [valor_em_brl] [moeda_destino]")
		os.Exit(1)
	}

	// Converte o valor recebido como texto para numero decimal.
	valueBRL, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "valor_em_brl deve ser um numero")
		os.Exit(1)
	}

	// Normaliza a moeda para aceitar entradas como "usd" ou "Usd".
	currency := strings.ToUpper(os.Args[2])
	rate, exists := rates[currency]
	if !exists {
		fmt.Fprintln(os.Stderr, "moeda_destino nao encontrada")
		os.Exit(1)
	}

	// Aplica a taxa da moeda escolhida e imprime somente o resultado final.
	convertedValue := valueBRL * rate
	fmt.Printf("%.2f\n", convertedValue)
}
