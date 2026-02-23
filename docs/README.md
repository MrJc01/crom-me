<p align="center">
  <strong>🌐 crom.me</strong><br/>
  <em>Subdomínios Gratuitos & Túneis HTTP para a Comunidade Crom</em>
</p>

---

## 📖 Visão Geral

O **crom.me** é uma plataforma open-source que oferece **subdomínios gratuitos** e **túneis HTTP reversos** para desenvolvedores, criadores de conteúdo e organizações que fazem parte do ecossistema Crom.

O projeto nasce de uma necessidade real: dar visibilidade a projetos independentes sem o custo de um domínio próprio ou a complexidade de configurar exposição pública para ambientes locais.

### O que o crom.me oferece

| Funcionalidade | Descrição |
|---|---|
| **Subdomínios Fixos** | Registre `*.crom.me` com provisionamento automatizado de certificados e roteamento via Cloudflare |
| **Edge Tunneling** | `*-free.crom.me` — Exponha portas locais à internet (estilo Ngrok) de forma instantânea via `crom-cli` |
| **Console de Engenharia** | Dashboard de controle de cotas, status de comunicação em tempo real e telemetria de requisições do túnel |
| **Command Center** | Gerenciamento administrativo (Admin Bypass, suspensão de usuários, análise e triagem de malha) |
| **API Programática** | Interaja com toda a plataforma (provisionamentos, endpoints, status e túnel) de forma programática |

---

## 🎯 Objetivos do Projeto

1. **Democratizar o acesso** a subdomínios para a comunidade de desenvolvedores brasileiros.
2. **Oferecer uma alternativa gratuita ao ngrok** via `free.crom.me`, com foco em performance e simplicidade.
3. **Manter a segurança e a reputação** do domínio principal através de curadoria e conformidade com a **LGPD**.
4. **Fomentar a comunidade Crom** premiando contribuições com slots adicionais de subdomínios.

---

## ⚙️ Requisitos Técnicos

### Para Desenvolvimento

| Requisito | Versão Mínima | Finalidade |
|---|---|---|
| **Go** | 1.22+ | Backend da API, CLI do Túnel e lógica de negócio |
| **PostgreSQL** | 15+ | Persistência de dados (usuários, domínios, quotas) |
| **Cloudflare Account** | — | Gerenciamento de DNS via API v4 |
| **Git** | 2.40+ | Controle de versão |

### Variáveis de Ambiente Necessárias

```bash
# Cloudflare
CLOUDFLARE_API_TOKEN=your_api_token
CLOUDFLARE_ZONE_ID=your_zone_id

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/cromme?sslmode=disable

# Auth
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret

# App
APP_SECRET=your_jwt_secret
APP_ENV=development
```

---

## 📂 Estrutura do Projeto

```
crom-me/
├── cmd/                  # Binários (API Server e CLI do Túnel)
│   ├── api/              # Entry point do servidor HTTP
│   └── tunnel/           # Entry point do cliente de túnel
├── internal/             # Lógica de negócio (não exportada)
│   ├── auth/             # OAuth GitHub + JWT
│   ├── cloudflare/       # Wrapper da API Cloudflare v4
│   ├── database/         # Migrations e queries SQL
│   ├── domain/           # Regras de negócio para subdomínios
│   └── tunnel/           # Lógica do túnel reverso
├── web/                  # Painel de gerenciamento (Frontend)
│   ├── static/           # CSS, JS, imagens
│   └── templates/        # Templates HTML
├── docs/                 # Documentação técnica (você está aqui)
├── go.mod
├── go.sum
└── Makefile
```

---

## 🤝 Como a Comunidade Pode Participar

### 🐛 Reportar Bugs & Abusos

Encontrou um subdomínio sendo usado para phishing ou spam? Abra uma [Issue](https://github.com/MrJc01/crom-me/issues) com a tag `abuse-report`.

### 💻 Contribuir com Código

1. Faça um **fork** do repositório.
2. Crie uma **branch** para sua feature: `git checkout -b feature/minha-feature`.
3. Faça **commit** das suas alterações: `git commit -m 'feat: adiciona minha feature'`.
4. Envie um **Pull Request** para a branch `main`.

> **Dica:** Contribuidores ativos ganham **slots extras de subdomínios**. Veja mais em [COMPLIANCE_AND_PARTNERSHIP.md](./COMPLIANCE_AND_PARTNERSHIP.md).

### 📣 Divulgar

Escreva um artigo, faça um vídeo ou poste nas redes sociais sobre como você está usando o `crom.me`. Isso ajuda o projeto a crescer organicamente.

---

## 📚 Documentação Adicional

| Documento | Descrição |
|---|---|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Arquitetura técnica, stack e fluxos de dados |
| [DATABASE.md](./DATABASE.md) | Esquema SQL e decisões de modelagem |
| [TUNNEL_SPECS.md](./TUNNEL_SPECS.md) | Especificações do túnel `free.crom.me` |
| [COMPLIANCE_AND_PARTNERSHIP.md](./COMPLIANCE_AND_PARTNERSHIP.md) | LGPD, regras de uso e sistema de parcerias |

---

## 📜 Licença

Este projeto adota a **Crom Sustainable Use License** (inspirada na licença Fair-Code do n8n). Você é livre para usar, modificar e hospedar o código para uso pessoal ou interno, **mas é estritamente proibido revendê-lo ou oferecê-lo como um SaaS comercial** sem autorização prévia.

📧 Para discussões sobre licenciamento comercial, entre em contato via: **mrj.crom@gmail.com**

Veja o texto completo no arquivo [LICENSE](../LICENSE).

---

<p align="center">
  Feito com 🖤 pela <strong>Comunidade Crom</strong>
</p>
