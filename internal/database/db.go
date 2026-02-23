package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB encapsula a conexão com o banco de dados via pool
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB cria e inicializa a conexão com o PostgreSQL
func NewDB(ctx context.Context) (*DB, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL não configurada")
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear config do banco: %w", err)
	}

	// Configurações recomendadas para performance
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	// Verifica a conexão (Ping)
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("falha no ping do banco: %w", err)
	}

	log.Println("✅ Conexão com o PostgreSQL estabelecida com sucesso")
	return &DB{Pool: pool}, nil
}

// Close fecha a conexão com o banco
func (db *DB) Close() {
	db.Pool.Close()
}
