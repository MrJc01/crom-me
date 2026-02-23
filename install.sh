#!/usr/bin/env bash
set -e

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  crom-cli installer — https://crom.me
#  Suporta: Linux, macOS, Windows (Git Bash/MSYS2)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Cores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

REPO="MrJc01/crom-me"
BINARY="crom-cli"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Instalador crom-cli${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# ──────────────────────────────────────────────────
# 1. Detectar Sistema Operacional
# ──────────────────────────────────────────────────
detect_os() {
    local os
    os="$(uname -s 2>/dev/null || echo "Unknown")"
    case "$os" in
        Linux*)         echo "linux" ;;
        Darwin*)        echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*|Windows_NT*)
                        echo "windows" ;;
        *)
            echo -e "${RED}❌ Sistema Operacional não suportado: $os${NC}" >&2
            echo -e "${RED}   Plataformas suportadas: Linux, macOS, Windows${NC}" >&2
            exit 1
            ;;
    esac
}

# ──────────────────────────────────────────────────
# 2. Detectar Arquitetura
# ──────────────────────────────────────────────────
detect_arch() {
    local arch
    arch="$(uname -m 2>/dev/null || echo "Unknown")"
    case "$arch" in
        x86_64|amd64)           echo "amd64" ;;
        arm64|aarch64)          echo "arm64" ;;
        armv7l|armhf)           echo "arm" ;;
        i386|i686)
            echo -e "${RED}❌ Arquitetura 32-bit não é suportada.${NC}" >&2
            exit 1
            ;;
        *)
            echo -e "${RED}❌ Arquitetura não suportada: $arch${NC}" >&2
            echo -e "${RED}   Arquiteturas suportadas: amd64, arm64${NC}" >&2
            exit 1
            ;;
    esac
}

# ──────────────────────────────────────────────────
# 3. Buscar URL da release mais recente
# ──────────────────────────────────────────────────
fetch_download_url() {
    local target_os="$1"
    local target_arch="$2"
    local asset_name="${BINARY}-${target_os}-${target_arch}"

    # Windows usa .exe
    if [ "$target_os" = "windows" ]; then
        asset_name="${asset_name}.exe"
    fi

    echo -e "${BLUE}🔍 Buscando a versão mais recente para ${YELLOW}${target_os}/${target_arch}${BLUE}...${NC}" >&2

    local api_response
    api_response=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)

    if [ -z "$api_response" ]; then
        echo -e "${RED}❌ Erro ao acessar a API do GitHub.${NC}" >&2
        echo -e "${RED}   Verifique sua conexão ou tente novamente.${NC}" >&2
        exit 1
    fi

    local url
    url=$(echo "$api_response" | grep "browser_download_url.*${asset_name}" | head -1 | cut -d '"' -f 4)

    if [ -z "$url" ]; then
        echo -e "${RED}❌ Binário não encontrado: ${asset_name}${NC}" >&2
        echo -e "${RED}   Verifique releases em: https://github.com/${REPO}/releases${NC}" >&2

        # Combinação windows/arm64 não existe
        if [ "$target_os" = "windows" ] && [ "$target_arch" = "arm64" ]; then
            echo -e "${YELLOW}   ⚠️  Windows ARM64 ainda não é suportado.${NC}" >&2
        fi
        exit 1
    fi

    echo "$url"
}

# ──────────────────────────────────────────────────
# 4. Download
# ──────────────────────────────────────────────────
download_file() {
    local url="$1"
    local dest="$2"

    echo -e "${BLUE}📥 Baixando de: ${NC}${url}"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$dest" "$url"
    else
        echo -e "${RED}❌ Nem 'curl' nem 'wget' estão instalados.${NC}" >&2
        exit 1
    fi

    if [ ! -s "$dest" ]; then
        echo -e "${RED}❌ Download falhou. O arquivo está vazio.${NC}" >&2
        exit 1
    fi
}

# ──────────────────────────────────────────────────
# 5. Instalar binário
# ──────────────────────────────────────────────────
install_binary() {
    local target_os="$1"
    local tmp_file="$2"
    local bin_name="$BINARY"

    if [ "$target_os" = "windows" ]; then
        bin_name="${BINARY}.exe"
    fi

    # ── Windows (Git Bash / MSYS2) ──
    if [ "$target_os" = "windows" ]; then
        local win_install_dir="$HOME/bin"
        mkdir -p "$win_install_dir"
        mv "$tmp_file" "$win_install_dir/$bin_name"

        echo -e "${GREEN}✅ Instalado em: ${win_install_dir}/${bin_name}${NC}"

        # Verificar PATH
        if [[ ":$PATH:" != *":$win_install_dir:"* ]]; then
            echo ""
            echo -e "${YELLOW}⚠️  $win_install_dir não está no seu PATH.${NC}"
            echo -e "   Adicione ao PATH do Windows:"
            echo -e "   ${BLUE}setx PATH \"%PATH%;%USERPROFILE%\\bin\"${NC}"
            echo -e "   Ou no Git Bash, adicione ao ~/.bashrc:"
            echo -e "   ${BLUE}export PATH=\"\$HOME/bin:\$PATH\"${NC}"
        fi
        return
    fi

    # ── Linux / macOS ──
    chmod +x "$tmp_file"

    local system_dir="/usr/local/bin"

    if [ -w "$system_dir" ]; then
        mv "$tmp_file" "$system_dir/$bin_name"
        echo -e "${GREEN}✅ Instalado em: ${system_dir}/${bin_name}${NC}"
    elif command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
        # Terminal interativo com sudo disponível
        echo -e "${YELLOW}🔐 Necessário permissão de superusuário para instalar em ${system_dir}${NC}"
        sudo mv "$tmp_file" "$system_dir/$bin_name"
        echo -e "${GREEN}✅ Instalado em: ${system_dir}/${bin_name}${NC}"
    else
        # Fallback: instalar localmente em ~/.local/bin
        local user_dir="$HOME/.local/bin"
        mkdir -p "$user_dir"
        mv "$tmp_file" "$user_dir/$bin_name"
        echo -e "${GREEN}✅ Instalado em: ${user_dir}/${bin_name}${NC}"

        if [[ ":$PATH:" != *":$user_dir:"* ]]; then
            echo ""
            echo -e "${YELLOW}⚠️  $user_dir não está no seu PATH.${NC}"
            echo -e "   Adicione ao seu ~/.bashrc ou ~/.zshrc:"
            echo -e "   ${BLUE}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
        fi
    fi
}

# ──────────────────────────────────────────────────
# MAIN
# ──────────────────────────────────────────────────

TARGET_OS=$(detect_os)
TARGET_ARCH=$(detect_arch)

echo -e "🖥️  Sistema: ${GREEN}${TARGET_OS}${NC} | Arquitetura: ${GREEN}${TARGET_ARCH}${NC}"
echo ""

# Buscar URL
DOWNLOAD_URL=$(fetch_download_url "$TARGET_OS" "$TARGET_ARCH")

# Preparar download
if [ "$TARGET_OS" = "windows" ]; then
    TMP_FILE=$(mktemp "${TEMP:-/tmp}/crom-cli-XXXXXX.exe")
else
    TMP_FILE=$(mktemp /tmp/crom-cli-XXXXXX)
fi

# Cleanup ao sair
trap "rm -f '$TMP_FILE'" EXIT

# Download
download_file "$DOWNLOAD_URL" "$TMP_FILE"

# Instalar
install_binary "$TARGET_OS" "$TMP_FILE"

# ── Mensagem final ──
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ crom-cli instalado com sucesso!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Para começar:"
echo -e "  1. Autentique-se:  ${BLUE}crom-cli auth login${NC}"
echo -e "  2. Exponha porta:  ${BLUE}crom-cli tunnel 8080${NC}"
echo ""

# Hint para Windows (PowerShell nativo)
if [ "$TARGET_OS" = "windows" ]; then
    echo -e "${YELLOW}💡 Dica para PowerShell nativo:${NC}"
    echo -e "   ${BLUE}irm https://crom.me/install.ps1 | iex${NC}"
    echo ""
fi
