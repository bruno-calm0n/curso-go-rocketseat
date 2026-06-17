# API de Usuarios com SQLite e Testes

API REST em Go para gerenciar usuarios com CRUD completo, persistencia em SQLite e testes automatizados.

## Como executar

```bash
go mod tidy
go run ./cmd/api
```

Servidor:

```bash
http://localhost:8080
```

## Como testar

```bash
go test ./...
go test ./... -cover
```

## Rotas

### Criar usuario

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d "{\"first_name\":\"Jane\",\"last_name\":\"Doe\",\"biography\":\"Tendo diversao estudando Go todos os dias\"}"
```

### Listar usuarios

```bash
curl http://localhost:8080/api/users
```

### Buscar usuario por ID

```bash
curl http://localhost:8080/api/users/{id}
```

### Atualizar usuario

```bash
curl -X PUT http://localhost:8080/api/users/{id} \
  -H "Content-Type: application/json" \
  -d "{\"first_name\":\"Jane\",\"last_name\":\"Silva\",\"biography\":\"Atualizando a biografia com texto valido\"}"
```

### Deletar usuario

```bash
curl -X DELETE http://localhost:8080/api/users/{id}
```

## Testes incluidos

- CRUD completo no SQLite
- Casos de usuario nao encontrado
- Persistencia apos reabrir o banco
- Migracao executando mais de uma vez
- Uso seguro de argumentos nas queries
- Endpoints HTTP de criacao, listagem, busca, atualizacao e remocao
- Validacoes de JSON e campos obrigatorios
- Respostas de erro em JSON
