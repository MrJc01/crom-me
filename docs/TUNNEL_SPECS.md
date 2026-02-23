# 🚇 TUNNEL_SPECS.md — Especificações do free.crom.me

> Serviço de túnel reverso HTTP gratuito sob o subdomínio `free.crom.me`.

---

## 1. O Que é o free.crom.me?

Semelhante ao [ngrok](https://ngrok.com), permite expor um servidor local (ex: `localhost:3000`) à internet via subdomínio temporário como `abc123.free.crom.me`.

| Feature | ngrok (Free) | free.crom.me |
|---|---|---|
| Subdomínio | Aleatório | Aleatório (personalizado para parceiros) |
| HTTPS | ✅ | ✅ (via Cloudflare) |
| Autenticação | ❌ | ✅ GitHub OAuth obrigatório |
| Custo | Grátis (limitado) | 100% Gratuito |

---

## 2. Arquitetura

```
          Internet → Cloudflare (*.free.crom.me Wildcard DNS)
                          │
                          ▼
                   ┌─────────────┐
                   │ Tunnel      │  ← VPS Central (Go)
                   │ Server      │
                   │ + Router    │  ← Mapeia subdomínio → ws_conn
                   └──────┬──────┘
                          │ WebSocket
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │ Client  │  │ Client  │  │ Client  │
       │ :3000   │  │ :8080   │  │ :5000   │
       └─────────┘  └─────────┘  └─────────┘
```

| Componente | Responsabilidade |
|---|---|
| **Tunnel Server** | Aceita conexões WebSocket, roteia tráfego HTTP |
| **Router Registry** | Mapeia subdomínio aleatório → conexão WebSocket ativa |
| **Tunnel CLI** | Abre conexão reversa e encaminha para `localhost` |

---

## 3. Fluxo de Tráfego

### Estabelecimento do Túnel

1. Usuário executa `crom-cli tunnel 3000`
2. CLI autentica via GitHub OAuth token
3. Server gera subdomínio aleatório (ex: `x7k9m2`)
4. WebSocket é estabelecido
5. `https://x7k9m2.free.crom.me` está pronto

### Requisição HTTP

1. Browser acessa `x7k9m2.free.crom.me`
2. Server extrai subdomínio do `Host` header
3. Busca no Registry: `x7k9m2 → ws_conn_42`
4. Encapsula HTTP request e envia via WebSocket
5. CLI recebe e faz forward para `localhost:3000`
6. Response volta pelo mesmo caminho

---

## 4. Binário CLI — `crom-cli`

| Especificação | Valor |
|---|---|
| **Linguagem** | Go 1.22+ |
| **Plataformas** | Linux, macOS, Windows (amd64, arm64) |
| **Tamanho** | < 10 MB (binário estático) |
| **Dependências** | Zero |

### Instalação

```bash
# Linux / macOS
curl -fsSL https://crom.me/install.sh | sh

# Via Go
go install github.com/MrJc01/crom-me/cmd/tunnel@latest
```

### Uso

```bash
# Autenticação (primeira vez)
crom-cli auth login

# Iniciar túnel na porta 3000
crom-cli tunnel 3000
# → https://x7k9m2.free.crom.me → localhost:3000

# Com verbose
crom-cli tunnel 8080 --verbose
```

### Estrutura do CLI

```go
package main

import (
    "context"
    "flag"
    "log"
    "os/signal"
    "syscall"

    "github.com/MrJc01/crom-me/internal/tunnel"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() { <-sigCh; cancel() }()

    client := tunnel.NewClient(tunnel.Config{
        ServerURL: "wss://tunnel.crom.me/ws",
        LocalPort: 3000,
    })

    if err := client.Connect(ctx); err != nil {
        log.Fatalf("Erro: %v", err)
    }
}
```

---

## 5. Protocolo WebSocket (JSON)

#### Register (Client → Server)
```json
{ "type": "register", "payload": { "token": "jwt_token", "local_port": 3000 } }
```

#### Registered (Server → Client)
```json
{ "type": "registered", "payload": { "subdomain": "x7k9m2", "url": "https://x7k9m2.free.crom.me" } }
```

#### HTTP Request (Server → Client)
```json
{ "type": "http_request", "payload": { "id": "uuid", "method": "GET", "path": "/api/users", "headers": {}, "body": "" } }
```

#### HTTP Response (Client → Server)
```json
{ "type": "http_response", "payload": { "id": "uuid", "status": 200, "headers": {}, "body": "{}" } }
```

---

## 6. Limites e Rate Limiting

| Recurso | Free | Parceiro |
|---|---|---|
| Túneis simultâneos | 1 | 3 |
| Duração | 24h | 72h |
| Requests/min | 60 | 300 |
| Body size | 1 MB | 10 MB |
| Bandwidth/dia | 1 GB | 10 GB |

---

## 7. Segurança

| Medida | Descrição |
|---|---|
| Auth obrigatória | GitHub OAuth para criar túneis |
| TLS end-to-end | Cloudflare HTTPS + WSS |
| Rate limiting | Token bucket por subdomínio |
| Auto-expire | Túneis expiram automaticamente |
| Blacklist de IPs | Brute force é bloqueado |

---

## 8. Roadmap

- [ ] **v0.1** — MVP: túnel HTTP (GET/POST)
- [ ] **v0.2** — WebSocket passthrough
- [ ] **v0.3** — Dashboard de métricas
- [ ] **v0.4** — Subdomínios persistentes (parceiros)
- [ ] **v1.0** — TCP tunneling (SSH, databases)
