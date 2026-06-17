# API com JWT + Refresh Token

API REST local para praticar autenticacao com access token JWT e refresh token persistido em SQLite.

## Rotas

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /me`

## Executar

```powershell
go run ./cmd/api
```

## Testes

```powershell
go test ./...
```
