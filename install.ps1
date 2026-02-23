# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  crom-cli installer para Windows (PowerShell)
#  Uso: irm https://crom.me/install.ps1 | iex
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

$ErrorActionPreference = "Stop"
$Repo = "MrJc01/crom-me"
$BinaryName = "crom-cli.exe"

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "  Instalador crom-cli para Windows" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

# 1. Detectar arquitetura
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Host "❌ Apenas sistemas 64-bit sao suportados." -ForegroundColor Red
    exit 1
}

$AssetName = "crom-cli-windows-${Arch}.exe"
Write-Host "🖥️  Sistema: windows | Arquitetura: $Arch" -ForegroundColor Green
Write-Host ""

# 2. Buscar release mais recente
Write-Host "🔍 Buscando versao mais recente..." -ForegroundColor Blue
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "crom-cli-installer" }
} catch {
    Write-Host "❌ Erro ao acessar API do GitHub: $_" -ForegroundColor Red
    exit 1
}

$Asset = $Release.assets | Where-Object { $_.name -eq $AssetName }
if (-not $Asset) {
    Write-Host "❌ Binario nao encontrado: $AssetName" -ForegroundColor Red
    Write-Host "   Verifique: https://github.com/$Repo/releases" -ForegroundColor Yellow
    exit 1
}

$DownloadUrl = $Asset.browser_download_url
Write-Host "📥 Baixando: $DownloadUrl" -ForegroundColor Blue

# 3. Download
$InstallDir = Join-Path $env:LOCALAPPDATA "crom-cli"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$DestPath = Join-Path $InstallDir $BinaryName
Invoke-WebRequest -Uri $DownloadUrl -OutFile $DestPath -UseBasicParsing

if (-not (Test-Path $DestPath)) {
    Write-Host "❌ Download falhou." -ForegroundColor Red
    exit 1
}

Write-Host "✅ Binario salvo em: $DestPath" -ForegroundColor Green

# 4. Adicionar ao PATH do usuario (se nao estiver)
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host ""
    Write-Host "✅ $InstallDir adicionado ao PATH do usuario." -ForegroundColor Green
    Write-Host "⚠️  Feche e reabra o terminal para aplicar." -ForegroundColor Yellow
} else {
    Write-Host "✅ $InstallDir ja esta no PATH." -ForegroundColor Green
}

# 5. Mensagem final
Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "  ✅ crom-cli instalado com sucesso!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host "  Para comecar:"
Write-Host "  1. Autentique-se:  " -NoNewline; Write-Host "crom-cli auth login" -ForegroundColor Cyan
Write-Host "  2. Exponha porta:  " -NoNewline; Write-Host "crom-cli tunnel 8080" -ForegroundColor Cyan
Write-Host ""
