# API de Lembretes

API REST em Go para gerenciar lembretes, usando Repository Pattern e Injecao de Dependencia.

## Como executar

```bash
go mod tidy
go run ./cmd/api
```

Servidor:

```bash
http://localhost:8080
```

## Rotas

### Criar lembrete

```bash
curl -X POST http://localhost:8080/notes \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Estudar Go\",\"content\":\"Praticar repository pattern\"}"
```

### Listar lembretes

```bash
curl http://localhost:8080/notes
```

### Buscar por ID

```bash
curl http://localhost:8080/notes/1
```

### Atualizar lembrete

```bash
curl -X PUT http://localhost:8080/notes/1 \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Estudar Go\",\"content\":\"Revisar injecao de dependencia\"}"
```

### Deletar lembrete

```bash
curl -X DELETE http://localhost:8080/notes/1
```

## Estrutura

```text
cmd/api                 ponto de entrada da aplicacao
internal/domain         entidades do dominio
internal/repository     interface e implementacao SQLite
internal/controller     handlers HTTP
```
