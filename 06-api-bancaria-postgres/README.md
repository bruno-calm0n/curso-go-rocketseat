# API Bancaria com PostgreSQL e CSRF

Servidor HTTP em Go para operacoes bancarias simples usando PostgreSQL local, migrations SQL e protecao CSRF.

## Preparar PostgreSQL local

Crie um banco local:

```bash
createdb banco_go
```

No PowerShell, configure a conexao:

```powershell
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/banco_go?sslmode=disable"
$env:CSRF_SECRET="troque-este-segredo"
```

As migrations em `migrations/` rodam automaticamente quando a API inicia.

## Executar

```bash
go run ./cmd/api
```

Servidor:

```text
http://localhost:8080
```

## CSRF

Busque um token antes de chamar rotas que alteram dados:

```bash
curl http://localhost:8080/csrf-token
```

Envie o token no header:

```text
X-CSRF-Token: seu-token
```

## Rotas

```text
POST   /conta
GET    /conta/{id}/saldo
POST   /conta/{id}/deposito
POST   /conta/{id}/saque
POST   /conta/transferencia
DELETE /conta/{id}
```

IDs retornados seguem o formato `pf_1` ou `pj_1`.

## Exemplos

Criar pessoa fisica:

```bash
curl -X POST http://localhost:8080/conta \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: TOKEN" \
  -d "{\"tipo\":\"pf\",\"renda_mensal\":5000,\"idade\":30,\"nome_completo\":\"Jane Doe\",\"celular\":\"11999999999\",\"email\":\"jane@example.com\",\"categoria\":\"cliente\",\"saldo\":100}"
```

Depositar:

```bash
curl -X POST http://localhost:8080/conta/pf_1/deposito \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: TOKEN" \
  -d "{\"valor\":50}"
```

Transferir:

```bash
curl -X POST http://localhost:8080/conta/transferencia \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: TOKEN" \
  -d "{\"origem_id\":\"pf_1\",\"destino_id\":\"pj_1\",\"valor\":30}"
```

## Testes

Testes locais que nao precisam do banco:

```bash
go test ./...
```

Smoke tests contra a API rodando:

```powershell
$env:SMOKE_TEST="1"
go test ./tests/smoke -v
```

Opcionalmente altere a URL:

```powershell
$env:SMOKE_BASE_URL="http://localhost:8080"
```
