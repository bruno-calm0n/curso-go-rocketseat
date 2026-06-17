# API de Tarefas com Paginacao, Filtros e Testes

API REST local para praticar CRUD com filtros, paginacao e testes HTTP.

## Rotas

- `POST /api/tasks`
- `GET /api/tasks?status=pending&priority=high&page=1&limit=10&sort=newest`
- `GET /api/tasks/{id}`
- `PUT /api/tasks/{id}`
- `DELETE /api/tasks/{id}`

## Executar

```powershell
go run ./cmd/api
```

## Testes

```powershell
go test ./...
```
