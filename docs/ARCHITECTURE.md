<![CDATA[# 🏛️ ARCHITECTURE.md — Arquitetura Técnica do crom.me

> Documento de referência para desenvolvedores que desejam entender a arquitetura interna do sistema.

---

## 1. Visão Geral da Stack

O crom.me é construído com foco em **performance**, **simplicidade** e **controle total** sobre a infraestrutura.

```
┌─────────────────────────────────────────────────────────────┐
│                      CROM.ME STACK                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────────┐    ┌──────────┐    ┌───────────────────┐     │
│   │  Frontend │───▶│  Go API  │───▶│  PostgreSQL 15+   │     │
│   │  (Web)   │    │  Server  │    │  (Persistência)   │     │
│   └──────────┘    └────┬─────┘    └───────────────────┘     │
│                        │                                     │
│                        ▼                                     │
│               ┌────────────────┐                             │
│               │  Cloudflare    │                             │
│               │  API v4 (DNS)  │                             │
│               └────────────────┘                             │
│                                                             │
│   ┌──────────┐    ┌──────────────┐                          │
│   │  CLI     │───▶│  Tunnel      │                          │
│   │  Client  │    │  Server      │                          │
│   └──────────┘    │ (WebSocket)  │                          │
│                   └──────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

### Componentes Principais

| Componente | Tecnologia | Responsabilidade |
|---|---|---|
| **API Server** | Go (`net/http` + router) | Endpoints REST, autenticação, lógica de negócio |
| **Frontend (Web)** | HTML/CSS/JS (Templates Go) | Painel de gerenciamento de subdomínios |
| **Banco de Dados** | PostgreSQL 15+ | Armazenamento de usuários, domínios, quotas e logs |
| **DNS Manager** | Cloudflare API v4 | Criação/remoção de registros DNS em tempo real |
| **Tunnel Server** | Go (WebSocket/TCP) | Roteamento de tráfego para `free.crom.me` |
| **Tunnel CLI** | Go (binário compilado) | Cliente que o usuário executa para expor sua máquina |

---

## 2. Cloudflare — API v4 e Wildcard DNS

### 2.1 Por que Cloudflare?

- **API robusta** para gerenciamento programático de DNS.
- **Proteção DDoS** gratuita na camada de proxy.
- **Propagação instantânea** (~5 segundos) para registros DNS.
- **SSL automático** via Universal SSL para todos os subdomínios.

### 2.2 Wildcard DNS

O crom.me utiliza dois registros **Wildcard** como estratégia de roteamento:

```dns
*.crom.me        →  CNAME  →  proxy.crom.me       (Servidor da API/Frontend)
*.free.crom.me   →  CNAME  →  tunnel.crom.me      (Servidor de Túnel)
```

**Vantagem:** Ao usar Wildcard, qualquer subdomínio criado **funciona instantaneamente** sem necessidade de criar um registro DNS individual. O roteamento é feito na camada de aplicação.

### 2.3 Integração com a API

O wrapper Go encapsula as operações mais comuns:

```go
package cloudflare

import (
    "context"
    "fmt"
    "net/http"
)

// Client encapsula a comunicação com a Cloudflare API v4
type Client struct {
    apiToken string
    zoneID   string
    http     *http.Client
    baseURL  string
}

// NewClient cria uma nova instância do cliente Cloudflare
func NewClient(apiToken, zoneID string) *Client {
    return &Client{
        apiToken: apiToken,
        zoneID:   zoneID,
        http:     &http.Client{},
        baseURL:  "https://api.cloudflare.com/client/v4",
    }
}

// CreateSubdomain registra um novo subdomínio via DNS record
func (c *Client) CreateSubdomain(ctx context.Context, subdomain, target string) error {
    endpoint := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, c.zoneID)
    // POST com tipo CNAME apontando para o target
    // ...
    return nil
}

// DeleteSubdomain remove um registro DNS pelo ID
func (c *Client) DeleteSubdomain(ctx context.Context, recordID string) error {
    endpoint := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, c.zoneID, recordID)
    // DELETE request
    // ...
    return nil
}
```

> **Nota:** Mesmo usando Wildcard, mantemos registros individuais no banco de dados para controle de inventário e auditoria.

---

## 3. Fluxo de Aprovação de Subdomínios

O processo de solicitação de subdomínio é **curado** para evitar abusos e manter a reputação do domínio principal.

### 3.1 Diagrama de Fluxo

```
Usuário                          Sistema                        Admin
  │                                │                              │
  │  1. Login via GitHub OAuth     │                              │
  │ ─────────────────────────────▶ │                              │
  │                                │                              │
  │  2. Preenche formulário        │                              │
  │     (CPF/CNPJ + motivo)        │                              │
  │ ─────────────────────────────▶ │                              │
  │                                │                              │
  │                                │  3. Valida documento         │
  │                                │     (formato + duplicidade)  │
  │                                │                              │
  │                                │  4. Verifica quota           │
  │                                │     disponível               │
  │                                │                              │
  │                                │  5. Cria solicitação         │
  │                                │     status = "pending"       │
  │                                │                              │
  │                                │  6. Notifica admin           │
  │                                │ ────────────────────────────▶│
  │                                │                              │
  │                                │                              │ 7. Aprova/Rejeita
  │                                │  8. Atualiza status          │◀──────────────
  │                                │                              │
  │                                │  9. Se aprovado:             │
  │                                │     - Cria DNS record        │
  │                                │     - Decrementa quota       │
  │                                │                              │
  │  10. Recebe notificação        │                              │
  │ ◀───────────────────────────── │                              │
```

### 3.2 Limites por Tipo de Conta

| Tipo | Documento | Limite Base | Máximo (com Parceria) | Validação |
|---|---|---|---|---|
| **Pessoa Física (PF)** | CPF | 2 subdomínios | Variável | Hash SHA-256 + Salt |
| **Pessoa Jurídica (PJ)** | CNPJ | 10 subdomínios | Variável | Hash SHA-256 + Salt |

### 3.3 Critérios de Aprovação

O administrador avalia os seguintes critérios antes de aprovar:

1. **Perfil GitHub ativo** — conta com histórico de atividade.
2. **Propósito legítimo** — portfólio, blog técnico, ferramenta open-source, landing page.
3. **Nome do subdomínio** — não pode estar na blacklist (`admin`, `api`, `login`, `suporte`, etc.).
4. **Documento válido** — formato correto e sem registro duplicado no sistema.

### 3.4 Blacklist de Subdomínios Reservados

```go
var ReservedSubdomains = []string{
    "admin", "api", "app", "auth", "blog",
    "cdn", "dashboard", "dev", "docs", "ftp",
    "git", "imap", "login", "mail", "ns1", "ns2",
    "pop", "smtp", "ssh", "staging", "status",
    "suporte", "support", "test", "tunnel", "vpn",
    "webmail", "www", "free",
}
```

---

## 4. Autenticação e Autorização

### 4.1 OAuth 2.0 via GitHub

O único método de autenticação é via **GitHub OAuth**. Isso garante:

- Verificação de identidade real (conta GitHub com histórico).
- Sem necessidade de gerenciar senhas.
- Acesso ao perfil público do usuário para validação.

### 4.2 Fluxo de Auth

```
Usuário           crom.me API          GitHub
  │                    │                  │
  │  GET /auth/github  │                  │
  │ ──────────────────▶│                  │
  │                    │  Redirect OAuth  │
  │                    │ ────────────────▶│
  │                    │                  │
  │  Consent Screen    │                  │
  │ ◀─────────────────────────────────────│
  │                    │                  │
  │  Callback + Code   │                  │
  │ ──────────────────▶│                  │
  │                    │  Exchange Token  │
  │                    │ ────────────────▶│
  │                    │                  │
  │                    │  Access Token    │
  │                    │ ◀────────────────│
  │                    │                  │
  │  JWT crom.me       │                  │
  │ ◀──────────────────│                  │
```

### 4.3 Roles do Sistema

| Role | Permissões |
|---|---|
| **user** | Gerenciar seus próprios subdomínios, ver quota |
| **admin** | Aprovar/rejeitar solicitações, gerenciar usuários, ver abuse reports |
| **system** | Operações automatizadas (limpeza, notificações) |

---

## 5. Infraestrutura de Deploy

### 5.1 Ambiente de Produção (Recomendado)

```
                    ┌──────────────────┐
                    │   Cloudflare     │
                    │   (DNS + Proxy)  │
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
     ┌──────────────┐ ┌──────────┐ ┌──────────────┐
     │  VPS #1      │ │  VPS #2  │ │  DB Server   │
     │  API Server  │ │  Tunnel  │ │  PostgreSQL   │
     │  + Frontend  │ │  Server  │ │  (Managed)    │
     └──────────────┘ └──────────┘ └──────────────┘
```

### 5.2 Desenvolvimento Local

```bash
# 1. Clone o repositório
git clone https://github.com/MrJc01/crom-me.git
cd crom-me

# 2. Configure as variáveis de ambiente
cp .env.example .env
# Edite o .env com suas credenciais

# 3. Suba o banco de dados
docker compose up -d postgres

# 4. Rode as migrations
make migrate-up

# 5. Inicie o servidor
make run-api
```

---

## 📚 Referências

- [Cloudflare API v4 Docs](https://developers.cloudflare.com/api/)
- [Go `net/http` Package](https://pkg.go.dev/net/http)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/15/)
- [OAuth 2.0 — GitHub Docs](https://docs.github.com/en/apps/oauth-apps)
]]>
