# Conversor de Moedas

CLI em Go para converter valores em BRL para outra moeda usando taxas fixas.

## Como executar

```bash
go run . 10 USD
```

Saida:

```bash
1.51
```

## Como gerar o executavel

```bash
go build -o convert
./convert 12 JPY
```

Saida:

```bash
195.48
```

## Moedas disponiveis

- USD
- EUR
- JPY
- GBP
- CHF
- AUD
