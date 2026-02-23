#!/usr/bin/env bash
set -e

# Cores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}Iniciando a instalação do crom-cli...${NC}"

# Detecta OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux*)     TARGET_OS="linux" ;;
    darwin*)    TARGET_OS="darwin" ;;
    *)          echo -e "${RED}Sistema Operacional não suportado: $OS${NC}"; exit 1 ;;
esac

# Detecta Arquitetura
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)     TARGET_ARCH="amd64" ;;
    arm64*|aarch64) TARGET_ARCH="arm64" ;;
    *)          echo -e "${RED}Arquitetura não suportada: $ARCH${NC}"; exit 1 ;;
esac

# Obtem a versão mais recente da API do GitHub
echo "Buscando a versão mais recente..."
LATEST_URL=$(curl -s https://api.github.com/repos/MrJc01/crom-me/releases/latest | grep "browser_download_url.*crom-cli-${TARGET_OS}-${TARGET_ARCH}\b" | cut -d : -f 2,3 | tr -d \" | xargs)

if [ -z "$LATEST_URL" ]; then
    echo -e "${RED}Erro: Não foi possível encontrar o binário para ${TARGET_OS}-${TARGET_ARCH}.${NC}"
    echo "Verifique as releases manuais em: https://github.com/MrJc01/crom-me/releases"
    exit 1
fi

TMP_FILE="/tmp/crom-cli"

echo "Baixando de: $LATEST_URL"
# Tenta com curl primeiro, depois wget
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$LATEST_URL" -o "$TMP_FILE"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMP_FILE" "$LATEST_URL"
else
    echo -e "${RED}Erro: 'curl' ou 'wget' não estão instalados.${NC}"
    exit 1
fi

if [ ! -s "$TMP_FILE" ]; then
    echo -e "${RED}Erro: A transferência falhou. O arquivo baixado está vazio ou não existe.${NC}"
    exit 1
fi

echo "Dando permissões de execução..."
chmod +x "$TMP_FILE"

INSTALL_DIR="/usr/local/bin"

if [ -w "$INSTALL_DIR" ]; then
    echo "Instalando em $INSTALL_DIR..."
    mv "$TMP_FILE" "$INSTALL_DIR/crom-cli"
else
    # Tenta usar sudo se disponível no terminal, mas num pipe `curl | bash` isso frequentemente falha.
    # Alternativa segura: instalar em ~/.local/bin
    USER_INSTALL_DIR="$HOME/.local/bin"
    echo "Sem permissão em $INSTALL_DIR. Instalando localmente em $USER_INSTALL_DIR..."
    mkdir -p "$USER_INSTALL_DIR"
    mv "$TMP_FILE" "$USER_INSTALL_DIR/crom-cli"
    
    # Avisar sobre o PATH
    if [[ ":$PATH:" != *":$USER_INSTALL_DIR:"* ]]; then
        echo -e "${RED}Atenção: $USER_INSTALL_DIR não está no seu PATH.${NC}"
        echo "Adicione a seguinte linha no seu ~/.bashrc ou ~/.zshrc:"
        echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
fi

echo -e "${GREEN}✅ Sucesso! O crom-cli foi instalado.${NC}"
echo ""
echo "Para começar, autentique-se executando:"
echo -e "  ${BLUE}crom-cli auth login${NC}"
echo ""
echo "Depois exponha qualquer porta local:"
echo -e "  ${BLUE}crom-cli tunnel 8080${NC}"
