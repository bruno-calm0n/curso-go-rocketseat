CREATE TABLE IF NOT EXISTS pessoa_fisica (
    id SERIAL PRIMARY KEY,
    renda_mensal NUMERIC(14, 2) NOT NULL DEFAULT 0,
    idade INTEGER NOT NULL,
    nome_completo VARCHAR(255) NOT NULL,
    celular VARCHAR(20) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    categoria VARCHAR(50) NOT NULL,
    saldo NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (saldo >= 0)
);

CREATE TABLE IF NOT EXISTS pessoa_juridica (
    id SERIAL PRIMARY KEY,
    faturamento NUMERIC(14, 2) NOT NULL DEFAULT 0,
    idade INTEGER NOT NULL,
    nome_fantasia VARCHAR(255) NOT NULL,
    celular VARCHAR(20) NOT NULL,
    email_corporativo VARCHAR(255) NOT NULL UNIQUE,
    categoria VARCHAR(50) NOT NULL,
    saldo NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (saldo >= 0)
);
