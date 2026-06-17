# API de Usuarios em Memoria

API REST em Go para gerenciar usuarios com CRUD completo e armazenamento em memoria.

## Como executar

```bash
go run ./cmd/api
```

Servidor:

```bash
http://localhost:8080
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

## Validacoes

- `first_name`: obrigatorio, minimo 2 e maximo 20 caracteres
- `last_name`: obrigatorio, minimo 2 e maximo 20 caracteres
- `biography`: obrigatoria, minimo 20 e maximo 450 caracteres
