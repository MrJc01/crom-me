.PHONY: build run test clean migrate-up migrate-down

# Variáveis
BINARY_NAME=crom-api
CLI_NAME=crom-cli

build:
	go build -o bin/$(BINARY_NAME) cmd/api/main.go
	go build -o bin/$(CLI_NAME) cmd/tunnel/main.go

run-api:
	go run cmd/api/main.go

run-cli:
	go run cmd/tunnel/main.go -port 3000

test:
	go test -v ./...

clean:
	rm -rf bin/
	go clean

# Database (exemplo usando migrate tool se instalada)
migrate-up:
	# migrate -path internal/database/migrations/ -database "$(DATABASE_URL)" -verbose up

migrate-down:
	# migrate -path internal/database/migrations/ -database "$(DATABASE_URL)" -verbose down

# Setup inicial
setup:
	go mod download
	cp .env.example .env
	@echo "✅ Setup concluído. Edite o arquivo .env com suas credenciais."
