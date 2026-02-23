# 📋 COMPLIANCE_AND_PARTNERSHIP.md — Conformidade & Parcerias

> Regras de uso, conformidade com a LGPD e sistema de incentivo por contribuições.

---

## 1. Regras de Uso

### 1.1 Uso Permitido

| Uso | Exemplo |
|---|---|
| ✅ Portfólio pessoal | `joao.crom.me` |
| ✅ Blog técnico | `blog-joao.crom.me` |
| ✅ Projeto open-source | `meu-app.crom.me` |
| ✅ Landing page de produto | `startup.crom.me` |
| ✅ API de desenvolvimento | `api-dev.crom.me` |
| ✅ Documentação de projeto | `docs-projeto.crom.me` |

### 1.2 Uso Proibido

| Uso | Consequência |
|---|---|
| ❌ Phishing / engenharia social | **Banimento imediato** + reporte às autoridades |
| ❌ Distribuição de malware | **Banimento imediato** + reporte ao CERT.br |
| ❌ Spam ou scraping abusivo | Suspensão + investigação |
| ❌ Conteúdo ilegal (CSAM, drogas) | **Banimento** + denúncia à Polícia Federal |
| ❌ Sites de apostas / cassino | Suspensão imediata |
| ❌ Proxy para evasão de bloqueios | Suspensão + investigação |
| ❌ Mineração de criptomoedas via túnel | Rate limit automático + suspensão |

### 1.3 Processo de Punição

```
Abuse Report → Investigação (48h) → Decisão
                                     ├── Dismissed (falso positivo)
                                     ├── Warning (primeira ocorrência leve)
                                     ├── Suspended (suspensão temporária)
                                     └── Banned (banimento + dados retidos para fins legais)
```

---

## 2. Conformidade com a LGPD

O crom.me opera em total conformidade com a **Lei Geral de Proteção de Dados (Lei 13.709/2018)**.

### 2.1 Dados Coletados

| Dado | Classificação LGPD | Finalidade | Armazenamento |
|---|---|---|---|
| **GitHub ID** | Dado pessoal | Autenticação | Texto puro |
| **Username** | Dado pessoal | Identificação | Texto puro |
| **E-mail** | Dado pessoal | Comunicação | Texto puro |
| **CPF** | Dado pessoal **sensível** | Anti-fraude e duplicidade | **Hash SHA-256 + Salt** |
| **CNPJ** | Dado pessoal | Anti-fraude e duplicidade | **Hash SHA-256 + Salt** |
| **IP de acesso** | Dado pessoal | Logs de segurança | Texto puro (rotação 90 dias) |

### 2.2 Bases Legais (Art. 7º da LGPD)

| Base Legal | Aplicação |
|---|---|
| **Consentimento** (Art. 7º, I) | Coleta de CPF/CNPJ no formulário de cadastro |
| **Legítimo Interesse** (Art. 7º, IX) | Logs de acesso para segurança da plataforma |
| **Cumprimento de Obrigação Legal** (Art. 7º, II) | Retenção de dados para responder a ordens judiciais |

### 2.3 Direitos do Titular (Art. 18)

Todo usuário pode exercer seus direitos enviando e-mail para `privacidade@crom.me`:

| Direito | Implementação |
|---|---|
| **Acesso** | Exportação completa dos dados em JSON |
| **Correção** | Atualização via painel ou solicitação |
| **Eliminação** | Exclusão de conta + todos os subdomínios associados |
| **Portabilidade** | Export em formato padronizado (JSON) |
| **Revogação** | Remoção do consentimento + exclusão de dados sensíveis |

### 2.4 Tratamento de Dados Sensíveis (CPF/CNPJ)

```go
// Fluxo de tratamento do CPF/CNPJ
//
// 1. Usuário digita o documento no formulário
// 2. Frontend valida o formato (máscara)
// 3. Documento é enviado via HTTPS (TLS 1.3)
// 4. Backend gera Salt aleatório (16 bytes)
// 5. Backend calcula Hash = SHA-256(salt + documento)
// 6. Apenas hash + salt são armazenados no PostgreSQL
// 7. Documento original NUNCA é persistido
```

> **⚠️ Importante:** Em caso de vazamento do banco de dados, os CPFs/CNPJs **não podem ser recuperados** a partir dos hashes. Isso é uma decisão de design intencional.

### 2.5 Retenção e Exclusão

| Cenário | Política |
|---|---|
| Conta ativa | Dados retidos enquanto a conta existir |
| Conta excluída | Dados pessoais removidos em 30 dias |
| Conta banida | Hash do documento retido por 5 anos (prevenção de reuso) |
| Logs de acesso | Rotação automática a cada 90 dias |

---

## 3. Sistema de Parcerias e Aumento de Limites

### 3.1 Limites Base

| Tipo | Limite Base | Documento |
|---|---|---|
| **Pessoa Física (PF)** | 2 subdomínios | CPF |
| **Pessoa Jurídica (PJ)** | 10 subdomínios | CNPJ |

### 3.2 Como Ganhar Slots Extras

O sistema de bônus é baseado em **contribuições verificáveis** para o ecossistema Crom:

| Nível | Critério | Bônus |
|---|---|---|
| 🥉 **Contributor** | PR aceito em qualquer repo da Crom | +2 slots |
| 🥈 **Sustentador** | Apoio financeiro mensal (Pix/GitHub Sponsors) | +5 slots |
| 🥇 **Educador** | Artigo/vídeo publicado sobre a crom.me | +3 slots |
| 💎 **Parceiro Oficial** | Acordo formal de parceria | Slots ilimitados |

### 3.3 Fluxo de Solicitação de Aumento

```
Usuário                        Admin
  │                              │
  │  Solicita aumento de slots   │
  │  + link da contribuição      │
  │ ────────────────────────────▶│
  │                              │
  │                              │  Verifica contribuição
  │                              │  (PR, artigo, pagamento)
  │                              │
  │                              │  Atualiza bonus_limit
  │  Notificação de aprovação    │  na tabela quotas
  │ ◀────────────────────────────│
```

### 3.4 Parcerias Oficiais

Organizações que desejam parcerias de longo prazo devem entrar em contato via `parcerias@crom.me`. Critérios:

| Critério | Descrição |
|---|---|
| **Alinhamento** | O projeto deve estar alinhado com os valores da Crom |
| **Atividade** | A organização deve ter atividade pública verificável |
| **Compromisso** | Acordo mínimo de 6 meses |
| **Contrapartida** | Divulgação do ecossistema Crom em seus canais |

### 3.5 Revogação de Bônus

| Situação | Ação |
|---|---|
| Contribuição removida (ex: PR revertido) | Bônus mantido |
| Apoio financeiro cancelado | Bônus removido após 30 dias |
| Violação das regras de uso | **Todos** os bônus revogados |

---

## 4. Contato

| Assunto | Canal |
|---|---|
| Privacidade e LGPD | `privacidade@crom.me` |
| Abuse Reports | `abuse@crom.me` ou [GitHub Issues](https://github.com/MrJc01/crom-me/issues) |
| Parcerias | `parcerias@crom.me` |
| Suporte Geral | `suporte@crom.me` |

---

<p align="center">
  <em>Última atualização: Fevereiro 2026</em>
</p>
