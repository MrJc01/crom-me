package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHubUser representa os dados retornados pela API do GitHub
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
}

// OAuthProvider encapsula a lógica de autenticação com o GitHub
type OAuthProvider struct {
	Config *oauth2.Config
}

// NewGitHubProvider inicializa o provedor de OAuth do GitHub
func NewGitHubProvider() *OAuthProvider {
	return &OAuthProvider{
		Config: &oauth2.Config{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			Endpoint:     github.Endpoint,
			Scopes:       []string{"user:email", "read:user"},
			RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
		},
	}
}

// GetAuthURL retorna a URL para redirecionar o usuário para o GitHub
func (p *OAuthProvider) GetAuthURL(state string) string {
	return p.Config.AuthCodeURL(state)
}

// ExchangeToken troca o código de autorização por um token de acesso
func (p *OAuthProvider) ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := p.Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("falha na troca de token: %w", err)
	}
	return token, nil
}

// GetUserProfile busca os dados do usuário no GitHub usando o token
func (p *OAuthProvider) GetUserProfile(ctx context.Context, token *oauth2.Token) (*GitHubUser, error) {
	client := p.Config.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar perfil: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status inesperado da API GitHub: %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("erro ao decodificar perfil: %w", err)
	}

	return &user, nil
}
