package database

import (
	"context"
	"fmt"

	"github.com/MrJc01/crom-me/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PostgresRepository implementa domain.Repository usando pgx
type PostgresRepository struct {
	db *DB
}

func NewPostgresRepository(db *DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, d *domain.Domain) error {
	query := `
		INSERT INTO domains (id, user_id, subdomain, target, record_type, status, purpose, project_name, contact_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		d.ID, d.UserID, d.Subdomain, d.Target, d.RecordType, d.Status, d.Purpose, d.ProjectName, d.ContactEmail, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error) {
	query := `SELECT id, user_id, subdomain, target, record_type, dns_record_id, status, purpose, project_name, contact_email, created_at, updated_at, expires_at FROM domains WHERE id = $1`
	var d domain.Domain
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.UserID, &d.Subdomain, &d.Target, &d.RecordType, &d.DNSRecordID, &d.Status, &d.Purpose, &d.ProjectName, &d.ContactEmail, &d.CreatedAt, &d.UpdatedAt, &d.ExpiresAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("domínio não encontrado")
	}
	return &d, err
}

func (r *PostgresRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Domain, error) {
	query := `SELECT id, user_id, subdomain, target, record_type, dns_record_id, status, purpose, project_name, contact_email, created_at, updated_at, expires_at FROM domains WHERE user_id = $1`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*domain.Domain
	for rows.Next() {
		var d domain.Domain
		err := rows.Scan(
			&d.ID, &d.UserID, &d.Subdomain, &d.Target, &d.RecordType, &d.DNSRecordID, &d.Status, &d.Purpose, &d.ProjectName, &d.ContactEmail, &d.CreatedAt, &d.UpdatedAt, &d.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		domains = append(domains, &d)
	}
	return domains, nil
}

// GetBySubdomain busca um domínio pelo nome do subdomínio para Proxy Pass
func (r *PostgresRepository) GetBySubdomain(ctx context.Context, subdomain string) (*domain.Domain, error) {
	query := `SELECT id, user_id, subdomain, target, record_type, dns_record_id, status, purpose, project_name, contact_email, created_at, updated_at, expires_at FROM domains WHERE subdomain = $1 LIMIT 1`
	var d domain.Domain
	err := r.db.Pool.QueryRow(ctx, query, subdomain).Scan(
		&d.ID, &d.UserID, &d.Subdomain, &d.Target, &d.RecordType, &d.DNSRecordID, &d.Status, &d.Purpose, &d.ProjectName, &d.ContactEmail, &d.CreatedAt, &d.UpdatedAt, &d.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	query := `UPDATE domains SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, status, id)
	return err
}

func (r *PostgresRepository) GetQuota(ctx context.Context, userID uuid.UUID) (*domain.Quota, error) {
	query := `SELECT user_id, base_limit, bonus_limit, used_slots FROM quotas WHERE user_id = $1`
	var q domain.Quota
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&q.UserID, &q.BaseLimit, &q.BonusLimit, &q.UsedSlots)
	if err == pgx.ErrNoRows {
		// Se não existe, cria uma quota padrão
		return r.createDefaultQuota(ctx, userID)
	}
	return &q, err
}

func (r *PostgresRepository) createDefaultQuota(ctx context.Context, userID uuid.UUID) (*domain.Quota, error) {
	// Aqui poderíamos checar o tipo do usuário (PF/PJ), mas simplificando para PF padrão por enquanto
	q := &domain.Quota{UserID: userID, BaseLimit: 2, BonusLimit: 0, UsedSlots: 0}
	query := `INSERT INTO quotas (user_id, base_limit, bonus_limit, used_slots) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Pool.Exec(ctx, query, q.UserID, q.BaseLimit, q.BonusLimit, q.UsedSlots)
	return q, err
}

func (r *PostgresRepository) IncrementUsedSlots(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE quotas SET used_slots = used_slots + 1, updated_at = NOW() WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

func (r *PostgresRepository) GetPendingRequests(ctx context.Context) ([]*domain.Domain, error) {
	query := `SELECT id, user_id, subdomain, target, record_type, dns_record_id, status, purpose, created_at, updated_at, expires_at FROM domains WHERE status = 'pending' ORDER BY created_at ASC`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*domain.Domain
	for rows.Next() {
		var d domain.Domain
		if err := rows.Scan(&d.ID, &d.UserID, &d.Subdomain, &d.Target, &d.RecordType, &d.DNSRecordID, &d.Status, &d.Purpose, &d.CreatedAt, &d.UpdatedAt, &d.ExpiresAt); err != nil {
			return nil, err
		}
		domains = append(domains, &d)
	}
	return domains, nil
}

// CreateUser insere um novo usuário e sua quota padrão de forma atômica (transação)
func (r *PostgresRepository) CreateUser(ctx context.Context, user *domain.User) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback seguro — é no-op se já commitou

	// 1. Insere o usuário
	queryUser := `
		INSERT INTO users (id, github_id, username, email, avatar_url, type, doc_hash, doc_salt, role, verified, bio, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err = tx.Exec(ctx, queryUser,
		user.ID, user.GitHubID, user.Username, user.Email, user.AvatarURL,
		user.Type, user.DocHash, user.DocSalt, user.Role, user.Verified,
		user.Bio, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("erro ao inserir usuário: %w", err)
	}

	// 2. Cria quota padrão (2 slots para PF)
	queryQuota := `
		INSERT INTO quotas (user_id, base_limit, bonus_limit, used_slots)
		VALUES ($1, $2, $3, $4)
	`
	baseLimit := 2 // PF padrão
	_, err = tx.Exec(ctx, queryQuota, user.ID, baseLimit, 0, 0)
	if err != nil {
		return fmt.Errorf("erro ao criar quota padrão: %w", err)
	}

	// 3. Commit atômico
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("erro ao commitar transação: %w", err)
	}

	return nil
}

// GetUserByGitHubID busca um usuário pelo ID do GitHub.
// Retorna (user, true, nil) se encontrado, (nil, false, nil) se não encontrado.
func (r *PostgresRepository) GetUserByGitHubID(ctx context.Context, githubID int64) (*domain.User, bool, error) {
	query := `
		SELECT id, github_id, username, email, avatar_url, type, doc_hash, doc_salt, role, verified, bio, created_at, updated_at
		FROM users WHERE github_id = $1
	`
	var u domain.User
	err := r.db.Pool.QueryRow(ctx, query, githubID).Scan(
		&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL,
		&u.Type, &u.DocHash, &u.DocSalt, &u.Role, &u.Verified,
		&u.Bio, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("erro ao buscar usuário por GitHub ID: %w", err)
	}
	return &u, true, nil
}

// ListUsers lista todos os usuários cadastrados (Admin)
func (r *PostgresRepository) ListUsers(ctx context.Context) ([]*domain.User, error) {
	query := `
		SELECT id, github_id, username, email, avatar_url, type, doc_hash, doc_salt, role, verified, bio, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar usuários: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL,
			&u.Type, &u.DocHash, &u.DocSalt, &u.Role, &u.Verified,
			&u.Bio, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("erro escanerndo usuário: %w", err)
		}
		users = append(users, &u)
	}
	return users, nil
}

// UpdateQuotaLimit atualiza o base_limit de um usuário (promoção a parceiro)
func (r *PostgresRepository) UpdateQuotaLimit(ctx context.Context, userID uuid.UUID, newBaseLimit int) error {
	query := `UPDATE quotas SET base_limit = $1, updated_at = NOW() WHERE user_id = $2`
	result, err := r.db.Pool.Exec(ctx, query, newBaseLimit, userID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar quota: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("usuário não encontrado para promoção")
	}
	return nil
}

// UpdateDNSRecordID salva o ID do registro Cloudflare no domínio
func (r *PostgresRepository) UpdateDNSRecordID(ctx context.Context, domainID uuid.UUID, recordID string) error {
	query := `UPDATE domains SET dns_record_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, recordID, domainID)
	return err
}

// DeleteByID remove um registro de domínio do banco (usado ao rejeitar solicitações)
func (r *PostgresRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM domains WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("erro ao deletar domínio: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("domínio não encontrado para exclusão")
	}
	return nil
}

// GetAllDomains retorna todos os domínios do sistema (Admin)
func (r *PostgresRepository) GetAllDomains(ctx context.Context) ([]*domain.Domain, error) {
	query := `SELECT id, user_id, subdomain, target, record_type, dns_record_id, status, purpose, project_name, contact_email, created_at, updated_at, expires_at FROM domains ORDER BY created_at DESC`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*domain.Domain
	for rows.Next() {
		var d domain.Domain
		err := rows.Scan(
			&d.ID, &d.UserID, &d.Subdomain, &d.Target, &d.RecordType, &d.DNSRecordID, &d.Status, &d.Purpose, &d.ProjectName, &d.ContactEmail, &d.CreatedAt, &d.UpdatedAt, &d.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		domains = append(domains, &d)
	}
	return domains, nil
}

// DecrementUsedSlots decrementa o used_slots de um usuário (ao remover domínio ativo)
func (r *PostgresRepository) DecrementUsedSlots(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE quotas SET used_slots = GREATEST(used_slots - 1, 0), updated_at = NOW() WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

// UpdateUserRole atualiza o role de um usuário (ban/unban/admin)
func (r *PostgresRepository) UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) error {
	query := `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Pool.Exec(ctx, query, role, userID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("usuário não encontrado")
	}
	return nil
}
