package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// contextKey é um tipo customizado para evitar colisão de chaves no context
type contextKey string

const (
	// ContextKeyClaims é a chave usada para armazenar os claims do JWT no context
	ContextKeyClaims contextKey = "auth_claims"
)

// User representa um usuário no sistema crom.me
type User struct {
	ID        uuid.UUID `json:"id"`
	GitHubID  int64     `json:"github_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Type      string    `json:"type"`      // 'PF' ou 'PJ'
	DocHash   string    `json:"-"`         // SHA-256 do documento (nunca expor via JSON)
	DocSalt   string    `json:"-"`         // Salt único por usuário (nunca expor via JSON)
	Role      string    `json:"role"`      // 'user', 'admin', 'system'
	Verified  bool      `json:"verified"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserRepository define a interface para persistência de usuários
type UserRepository interface {
	// CreateUser insere um novo usuário e sua quota padrão (2 slots) de forma atômica (TX)
	CreateUser(ctx context.Context, user *User) error

	// GetUserByGitHubID busca um usuário pelo ID do GitHub.
	// Retorna (user, true, nil) se encontrado, (nil, false, nil) se não encontrado.
	GetUserByGitHubID(ctx context.Context, githubID int64) (*User, bool, error)

	// ListUsers lista todos os usuários cadastrados (Admin)
	ListUsers(ctx context.Context) ([]*User, error)

	// UpdateUserRole atualiza o role de um usuário (ban/unban/admin)
	UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) error
}
