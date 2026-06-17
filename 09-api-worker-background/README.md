# API com Worker em Background

API REST local para praticar processamento assincrono simples. A requisicao cria um job pendente e o worker processa em segundo plano.

## Rotas

- `POST /jobs`
- `GET /jobs`
- `GET /jobs/{id}`

## Executar

```powershell
go run ./cmd/api
```

## Testes

```powershell
go test ./...
```
