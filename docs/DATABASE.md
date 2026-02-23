<![CDATA[# 🗄️ DATABASE.md — Esquema do Banco de Dados

> Documentação completa do esquema SQL do crom.me, incluindo decisões de modelagem e estratégias de segurança para dados sensíveis.

---

## 1. Visão Geral do Modelo de Dados

```
┌─────────────┐       ┌──────────────┐
│   users     │       │   domains    │
│─────────────│       │──────────────│
│ id (PK)     │──┐    │ id (PK)      │
│ github_id   │  │    │ user_id (FK) │──┐
│ username    │  ├───▶│ subdomain    │  │
│ email       │  │    │ target       │  │
│ type        │  │    │ status       │  │
│ doc_hash    │  │    │ dns_record_id│  │
│ doc_salt    │  │    └──────────────┘  │
│ role        │  │                      │
└─────────────┘  │    ┌──────────────┐  │
                 │    │   quotas     │  │
                 │    │──────────────│  │
                 └───▶│ user_id (FK) │  │
                      │ base_limit   │  │
                      │ bonus_limit  │  │
                      └──────────────┘  │
                                        │
                      ┌───────────────┐ │
                      │ abuse_reports │ │
                      │───────────────│ │
                      │ id (PK)       │ │
                      │ domain_id(FK) │◀┘
                      │ reporter_ip   │
                      │ reason        │
                      │ status        │
                      └───────────────┘
```

---

## 2. Tabelas

### 2.1 `users` — Usuários do Sistema

Armazena os dados de autenticação e identificação de cada usuário registrado.

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id     BIGINT UNIQUE NOT NULL,
    username      VARCHAR(39) NOT NULL,          -- Limite do GitHub
    email         VARCHAR(255),
    avatar_url    TEXT,
    type          VARCHAR(2) NOT NULL             -- 'PF' ou 'PJ'
                  CHECK (type IN ('PF', 'PJ')),
    doc_hash      VARCHAR(64) NOT NULL,           -- SHA-256 do documento
    doc_salt      VARCHAR(32) NOT NULL,           -- Salt único por usuário
    role          VARCHAR(10) NOT NULL DEFAULT 'user'
                  CHECK (role IN ('user', 'admin', 'system')),
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    bio           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índices
CREATE UNIQUE INDEX idx_users_github_id ON users(github_id);
CREATE INDEX idx_users_doc_hash ON users(doc_hash);
CREATE INDEX idx_users_type ON users(type);
```

#### Campos Importantes

| Campo | Tipo | Descrição |
|---|---|---|
| `github_id` | `BIGINT` | ID único do GitHub, usado como chave de autenticação |
| `type` | `VARCHAR(2)` | `PF` para Pessoa Física, `PJ` para Pessoa Jurídica |
| `doc_hash` | `VARCHAR(64)` | Hash SHA-256 do CPF ou CNPJ (nunca texto puro) |
| `doc_salt` | `VARCHAR(32)` | Salt aleatório para tornar o hash único |
| `role` | `VARCHAR(10)` | Nível de acesso: `user`, `admin` ou `system` |
| `verified` | `BOOLEAN` | Se o documento foi verificado por um administrador |

---

### 2.2 `domains` — Subdomínios Registrados

Cada registro representa um subdomínio ativo ou pendente no sistema.

```sql
CREATE TABLE domains (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subdomain     VARCHAR(63) UNIQUE NOT NULL,    -- RFC 1035: máx 63 chars
    target        VARCHAR(255) NOT NULL,           -- IP ou CNAME de destino
    record_type   VARCHAR(5) NOT NULL DEFAULT 'CNAME'
                  CHECK (record_type IN ('A', 'AAAA', 'CNAME', 'TXT')),
    dns_record_id VARCHAR(32),                     -- ID do registro na Cloudflare
    status        VARCHAR(10) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'active', 'suspended', 'rejected')),
    purpose       TEXT NOT NULL,                   -- Motivo de uso (formulário)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ                      -- NULL = sem expiração
);

-- Índices
CREATE UNIQUE INDEX idx_domains_subdomain ON domains(subdomain);
CREATE INDEX idx_domains_user_id ON domains(user_id);
CREATE INDEX idx_domains_status ON domains(status);
```

#### Status do Subdomínio

| Status | Significado |
|---|---|
| `pending` | Aguardando aprovação do administrador |
| `active` | Aprovado e com registro DNS ativo na Cloudflare |
| `suspended` | Suspenso por violação de regras ou abuse report |
| `rejected` | Rejeitado pelo administrador |

---

### 2.3 `quotas` — Limites de Subdomínios

Controla quantos subdomínios cada usuário pode registrar, incluindo bônus de parcerias.

```sql
CREATE TABLE quotas (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_limit    INT NOT NULL DEFAULT 2,          -- PF=2, PJ=10
    bonus_limit   INT NOT NULL DEFAULT 0,          -- Slots extras (parcerias)
    used_slots    INT NOT NULL DEFAULT 0,          -- Subdomínios ativos
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_used_slots CHECK (used_slots >= 0),
    CONSTRAINT chk_limits CHECK (used_slots <= base_limit + bonus_limit)
);

-- Índice
CREATE UNIQUE INDEX idx_quotas_user_id ON quotas(user_id);
```

#### Lógica de Limites

```
Slots Disponíveis = base_limit + bonus_limit - used_slots
```

| Tipo Conta | `base_limit` | `bonus_limit` (Padrão) | Máximo Dinâmico |
|---|---|---|---|
| PF | 2 | 0 | base + bônus de contribuição |
| PJ | 10 | 0 | base + bônus de parceria |

---

### 2.4 `abuse_reports` — Denúncias de Abuso

Permite que a comunidade reporte subdomínios usados para atividades maliciosas.

```sql
CREATE TABLE abuse_reports (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id     UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    reporter_ip   INET NOT NULL,                   -- IP de quem reportou
    reason        TEXT NOT NULL,
    evidence_url  TEXT,                             -- Screenshot ou link
    status        VARCHAR(12) NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'investigating', 'resolved', 'dismissed')),
    admin_notes   TEXT,                             -- Notas internas do admin
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);

-- Índices
CREATE INDEX idx_abuse_reports_domain_id ON abuse_reports(domain_id);
CREATE INDEX idx_abuse_reports_status ON abuse_reports(status);
```

---

## 3. Por Que Usamos Hashing para CPF/CNPJ?

### 3.1 O Problema

O CPF e o CNPJ são **dados pessoais sensíveis** protegidos pela **Lei Geral de Proteção de Dados (LGPD — Lei 13.709/2018)**. Armazená-los em texto puro cria os seguintes riscos:

| Risco | Impacto |
|---|---|
| **Vazamento de dados** | Exposição massiva de documentos em caso de breach |
| **Responsabilidade legal** | Multas de até 2% do faturamento pela ANPD |
| **Engenharia social** | Documento pode ser usado para fraudes em outros serviços |

### 3.2 A Solução: Hash com Salt

Utilizamos **SHA-256 com Salt único por usuário** para armazenar os documentos. Isso garante:

```go
package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// GenerateSalt cria um salt aleatório de 16 bytes
func GenerateSalt() (string, error) {
    salt := make([]byte, 16)
    _, err := rand.Read(salt)
    if err != nil {
        return "", fmt.Errorf("falha ao gerar salt: %w", err)
    }
    return hex.EncodeToString(salt), nil
}

// HashDocument gera o hash SHA-256 do documento com salt
func HashDocument(document, salt string) string {
    data := fmt.Sprintf("%s:%s", salt, document)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// VerifyDocument verifica se um documento corresponde ao hash armazenado
func VerifyDocument(document, salt, storedHash string) bool {
    return HashDocument(document, salt) == storedHash
}
```

### 3.3 Fluxo de Verificação de Duplicidade

```
Novo Cadastro                         Sistema
    │                                    │
    │  CPF: 123.456.789-00              │
    │ ──────────────────────────────────▶│
    │                                    │
    │                                    │  1. Gera salt aleatório
    │                                    │  2. Hash = SHA-256(salt + CPF)
    │                                    │  3. Busca doc_hash no banco
    │                                    │
    │                                    │  Se hash existe:
    │  ✗ Documento já cadastrado         │     → Rejeita
    │ ◀──────────────────────────────────│
    │                                    │
    │                                    │  Se hash não existe:
    │  ✓ Cadastro aceito                 │     → Armazena hash + salt
    │ ◀──────────────────────────────────│
```

> **⚠️ Nota sobre Duplicidade:** Como cada usuário tem um salt único, dois hashes do mesmo CPF serão **diferentes**. Para checar duplicidade, iteramos sobre os registros existentes e recalculamos: `SHA-256(salt_existente + novo_cpf) == doc_hash_existente`. Uma alternativa mais eficiente é usar um **hash global sem salt** exclusivamente para checagem de duplicidade, armazenado em uma coluna separada (`doc_global_hash`).

### 3.4 Alternativa: Criptografia Simétrica (AES-256)

Se houver necessidade legal de **recuperar o documento** (ex: ordem judicial), podemos usar **AES-256-GCM** ao invés de hash. A chave de criptografia ficaria em um **HSM** ou **Vault** separado da aplicação.

| Método | Pode Recuperar? | Segurança em Breach | Uso Recomendado |
|---|---|---|---|
| SHA-256 + Salt | ❌ Não | ✅ Máxima | Checagem de duplicidade |
| AES-256-GCM | ✅ Sim (com chave) | ⚠️ Depende do Vault | Obrigações legais |

---

## 4. Migrations

As migrations são gerenciadas via arquivos SQL numerados em `internal/database/migrations/`:

```
internal/database/migrations/
├── 001_create_users.up.sql
├── 001_create_users.down.sql
├── 002_create_domains.up.sql
├── 002_create_domains.down.sql
├── 003_create_quotas.up.sql
├── 003_create_quotas.down.sql
├── 004_create_abuse_reports.up.sql
└── 004_create_abuse_reports.down.sql
```

### Executar Migrations

```bash
# Aplicar todas as migrations pendentes
make migrate-up

# Reverter a última migration
make migrate-down

# Ver status das migrations
make migrate-status
```

---

## 5. Queries Frequentes

### Subdomínios ativos de um usuário

```sql
SELECT d.subdomain, d.target, d.record_type, d.created_at
FROM domains d
WHERE d.user_id = $1
  AND d.status = 'active'
ORDER BY d.created_at DESC;
```

### Slots disponíveis

```sql
SELECT
    q.base_limit + q.bonus_limit AS total_limit,
    q.used_slots,
    (q.base_limit + q.bonus_limit - q.used_slots) AS available_slots
FROM quotas q
WHERE q.user_id = $1;
```

### Abuse reports pendentes (Admin)

```sql
SELECT
    ar.id,
    d.subdomain,
    u.username AS owner,
    ar.reason,
    ar.created_at
FROM abuse_reports ar
JOIN domains d ON ar.domain_id = d.id
JOIN users u ON d.user_id = u.id
WHERE ar.status = 'open'
ORDER BY ar.created_at ASC;
```
]]>
