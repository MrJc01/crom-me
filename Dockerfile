# Estágio 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Instalar dependências necessárias para a build
RUN apk add --no-cache git

# Baixar módulos com cache eficiente
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fonte
COPY . .

# Compilar binários (otimizado e seguro reduzindo tamanho com ldflags)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/crom-api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/crom-cli cmd/tunnel/main.go

# Estágio 2: Execução
FROM alpine:3.19

# Adicionar root certificates ca-certificates (vital para client http Cloudflare/Discord/OAuth)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copiar os binários minificados do builder
COPY --from=builder /app/bin/crom-api /app/bin/crom-api

# Copiar os templates HTML e assets
COPY --from=builder /app/web /app/web

# Copiar o script de instalação do CLI
COPY --from=builder /app/install.sh /app/install.sh

# Expõe a porta principal da API
EXPOSE 8080

# Define o comando de entrada
CMD ["/app/bin/crom-api"]
