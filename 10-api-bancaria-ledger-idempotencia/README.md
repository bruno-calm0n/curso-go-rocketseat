# API Bancaria com Ledger e Idempotencia

API REST local para praticar consistencia financeira. O saldo nao fica salvo em uma coluna: ele e calculado a partir de lancamentos imutaveis no ledger.

## Rotas

- `POST /accounts`
- `GET /accounts/{id}/balance`
- `GET /accounts/{id}/ledger`
- `POST /accounts/{id}/deposit`
- `POST /accounts/{id}/withdraw`
- `POST /transfers`

Todas as rotas `POST` exigem o header:

```text
Idempotency-Key: uma-chave-unica
```

## Executar

```powershell
go run ./cmd/api
```

## Testes

```powershell
go test ./...
```
