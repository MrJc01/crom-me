
## 🚀 Guia de Deploy: crom.me

### 1. Preparação Inicial (Primeira Vez)

Antes de subir os containers, certifique-se de que as configurações externas estão alinhadas:

* **GitHub OAuth**: Configure as URLs no painel de desenvolvedor do GitHub como `https://crom.me` (Homepage) e `https://crom.me/auth/callback` (Redirect URI).
* **Cloudflare**:
* **DNS**: Crie um registro **A** apontando para o IP da VPS e um **CNAME Wildcard** (`*`) apontando para `@`.
* **Origin Rules**: Crie uma regra para redirecionar o tráfego do hostname `crom.me` para a porta **8443** de destino.
* **SSL**: Defina o modo como **Flexible**.



#### Passo a Passo no Servidor:

1. **Clone o Repositório**:
```bash
git clone https://github.com/MrJc01/crom-me.git
cd crom-me

```


2. **Configuração de Ambiente**:
Copie o arquivo de exemplo e preencha as credenciais reais:
```bash
cp .env.example .env
nano .env

```


* Certifique-se de que `PORT=8080` e `GITHUB_REDIRECT_URL=https://crom.me/auth/callback`.


3. **Suba os Containers**:
```bash
docker-compose up -d --build

```


4. **Execute as Migrations**:
Como é a primeira vez, crie as tabelas manualmente no banco de dados:
```bash
docker exec -i crom_db psql -U cromme_user -d cromme < internal/database/migrations/001_create_users.up.sql
docker exec -i crom_db psql -U cromme_user -d cromme < internal/database/migrations/002_create_domains.up.sql
docker exec -i crom_db psql -U cromme_user -d cromme < internal/database/migrations/003_create_quotas.up.sql
docker exec -i crom_db psql -U cromme_user -d cromme < internal/database/migrations/004_create_abuse_reports.up.sql

```


5. **Promoção a Admin**:
Acesse `https://crom.me` no navegador, faça o login via GitHub e depois promova seu usuário no terminal:
```bash
docker exec -it crom_db psql -U cromme_user -d cromme -c "UPDATE users SET role = 'admin' WHERE username = 'SeuUsuarioGitHub';"

```



---

### 🔄 Atualizando o Sistema

Sempre que houver novos commits no repositório, siga estes passos:

1. **Atualize o Código**:
```bash
git pull origin main

```


2. **Recompile e Reinicie**:
```bash
docker-compose down
docker-compose up -d --build

```


3. **Verifique novas Migrations**:
Se houver novos arquivos `.sql` na pasta `internal/database/migrations`, execute-os individualmente seguindo o padrão do passo 4 da instalação inicial.
4. **Validar Saúde**:
Rode o script de healthcheck para confirmar se a API e o DB estão comunicando na porta correta:
```bash
chmod +x healthcheck.sh
./healthcheck.sh

```



---

### 🛠️ Manutenção Útil

* **Logs em Tempo Real**: `docker-compose logs -f api`
* **Reiniciar Nginx**: Se mudar o `nginx.conf`, use `docker restart crom_nginx`.
* **Acesso ao Banco**: `docker exec -it crom_db psql -U cromme_user -d cromme`
