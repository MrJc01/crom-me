#!/usr/bin/env bash

# Cores para o output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}>> Iniciando Health Check do crom.me...${NC}\n"

# 1. Verifica se a porta 9091 (API Go) está respondendo
echo -n "Verificando API (Porta 9091): "
if curl -s http://localhost:9091 > /dev/null; then
  echo -e "${GREEN}[OK] O servidor go está respondendo na porta 9091.${NC}"
else
  echo -e "${RED}[FALHA] Não foi possível conectar ao servidor na porta 9091.${NC}"
  echo "Dica: Verifique os logs do container 'api' (docker-compose logs api)."
fi

# 2. Verifica se o PostgreSQL está aceitando conexões na porta 5435
echo -n "Verificando PostgreSQL (Porta 5435): "
if nc -z localhost 5435 2>/dev/null; then
  echo -e "${GREEN}[OK] O PostgreSQL está rodando e aceitando conexões na porta 5435.${NC}"
else
  echo -e "${RED}[FALHA] O PostgreSQL não está escutando na porta 5435.${NC}"
  echo "Dica: Verifique se o container 'db' está rodando (docker-compose ps)."
fi

# 3. Verifica se as migrations podem ser necessárias (isso é mais um aviso pro DB no docker-compose)
echo -e "\n${YELLOW}>> Verificações adicionais:${NC}"
echo "→ Lembre-se de rodar suas tabelas (migrations) caso o banco seja novo e não tenha entrypoint automático."
echo "→ Certifique-se de que configurou o GitHub OAuth Client ID no seu arquivo .env"

echo -e "\n${GREEN}Healthcheck finalizado.${NC}"
