package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/MrJc01/crom-me/internal/auth"
	"github.com/MrJc01/crom-me/internal/database"
	"github.com/MrJc01/crom-me/internal/domain"
	"github.com/google/uuid"
)

// AuthHandler gerencia os endpoints de autenticação OAuth
type AuthHandler struct {
	oauth *auth.OAuthProvider
	repo  *database.PostgresRepository
}

// NewAuthHandler cria um novo handler de autenticação
func NewAuthHandler(oauth *auth.OAuthProvider, repo *database.PostgresRepository) *AuthHandler {
	return &AuthHandler{oauth: oauth, repo: repo}
}

// generateState cria um token aleatório para proteção CSRF no fluxo OAuth
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HandleLogin redireciona o usuário para a tela de autorização do GitHub
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		log.Printf("❌ Erro ao gerar state CSRF: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	// Salva state em cookie temporário para validação no callback
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutos
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Se veio do CLI, salva a URL de redirect para o callback
	if cliRedirect := r.URL.Query().Get("cli_redirect"); cliRedirect != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "cli_redirect",
			Value:    cliRedirect,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	url := h.oauth.GetAuthURL(state)
	log.Printf("🔑 Redirecionando para GitHub OAuth: %s", r.RemoteAddr)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// HandleCallback processa o retorno do GitHub após autorização
func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Validar state CSRF
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		log.Printf("⚠️ Cookie oauth_state ausente: %v", err)
		http.Error(w, "Estado de autenticação inválido", http.StatusBadRequest)
		return
	}

	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		log.Printf("⚠️ State CSRF inválido. Esperado: %s, Recebido: %s", stateCookie.Value, queryState)
		http.Error(w, "Falha na validação CSRF", http.StatusForbidden)
		return
	}

	// Limpa o cookie de state
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// 2. Pegar o authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("⚠️ Authorization code ausente no callback")
		http.Error(w, "Código de autorização ausente", http.StatusBadRequest)
		return
	}

	// 3. Trocar code por access token
	token, err := h.oauth.ExchangeToken(ctx, code)
	if err != nil {
		log.Printf("❌ Falha na troca de token: %v", err)
		http.Error(w, "Falha na autenticação com GitHub", http.StatusInternalServerError)
		return
	}

	// 4. Buscar perfil do usuário no GitHub
	ghUser, err := h.oauth.GetUserProfile(ctx, token)
	if err != nil {
		log.Printf("❌ Falha ao buscar perfil GitHub: %v", err)
		http.Error(w, "Falha ao obter dados do GitHub", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ GitHub User obtido: ID=%d, Login=%s", ghUser.ID, ghUser.Login)

	// 5. Verificar se o usuário já existe no banco
	existingUser, found, err := h.repo.GetUserByGitHubID(ctx, ghUser.ID)
	if err != nil {
		log.Printf("❌ Erro ao buscar usuário no banco: %v", err)
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}

	var userID string
	var userRole string

	if found {
		// Usuário já existe
		userID = existingUser.ID.String()
		userRole = existingUser.Role
		log.Printf("👤 Usuário existente: ID=%s, Role=%s", userID, userRole)
	} else {
		role := "user"
		if ghUser.Login == "MrJc01" {
			role = "admin"
		}

		// 6. Criar novo usuário (tipo PF, com placeholders para doc)
		newUser := &domain.User{
			ID:        uuid.New(),
			GitHubID:  ghUser.ID,
			Username:  ghUser.Login,
			Email:     ghUser.Email,
			AvatarURL: ghUser.AvatarURL,
			Type:      "PF",
			DocHash:   "pending_verification", // Placeholder até validação LGPD
			DocSalt:   generateSalt(),
			Role:      role,
			Verified:  false,
			Bio:       ghUser.Bio,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := h.repo.CreateUser(ctx, newUser); err != nil {
			log.Printf("❌ Erro ao criar usuário: %v", err)
			http.Error(w, "Erro ao registrar usuário", http.StatusInternalServerError)
			return
		}

		userID = newUser.ID.String()
		userRole = newUser.Role
		log.Printf("🆕 Novo usuário criado: ID=%s, Login=%s, Role=%s", userID, ghUser.Login, userRole)
	}

	// 7. Gerar JWT
	jwtToken, err := auth.GenerateToken(userID, userRole)
	if err != nil {
		log.Printf("❌ Erro ao gerar JWT: %v", err)
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	// 8. Salvar JWT em cookie seguro
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    jwtToken,
		Path:     "/",
		MaxAge:   259200, // 72 horas (alinhado com JWT expiry)
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true, // Habilitar quando em HTTPS (produção)
	})

	// Se veio do CLI, redireciona para o mini server local com o token
	if cliCookie, err := r.Cookie("cli_redirect"); err == nil && cliCookie.Value != "" {
		// Limpa o cookie
		http.SetCookie(w, &http.Cookie{
			Name: "cli_redirect", Value: "", Path: "/", MaxAge: -1,
		})
		redirectURL := cliCookie.Value + "?token=" + jwtToken
		log.Printf("🎉 Login CLI concluído para %s, redirecionando para CLI", ghUser.Login)
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	log.Printf("🎉 Login concluído para %s, redirecionando para /dashboard", ghUser.Login)
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// HandleLogout limpa o cookie de sessão
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	log.Printf("👋 Logout executado: %s", r.RemoteAddr)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleMe retorna os dados do usuário autenticado (via JWT no cookie)
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user_id": claims.UserID,
		"role":    claims.Role,
	})
}

// generateSalt cria um salt aleatório de 16 bytes (32 caracteres hex)
func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b) // Em caso de erro, retorna bytes zerados (aceitável para placeholder)
	return hex.EncodeToString(b)
}

// ListUsersHandler retorna todos os usuários cadastrados (Admin)
func (h *AuthHandler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if users == nil {
		users = []*domain.User{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// BanUserHandler define o role de um usuário como "banned"
func (h *AuthHandler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateUserRole(r.Context(), id, "banned"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "banned", "message": "Usuário banido com sucesso"})
}

// UnbanUserHandler restaura o role de um usuário para "user"
func (h *AuthHandler) UnbanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateUserRole(r.Context(), id, "user"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unbanned", "message": "Usuário desbanido com sucesso"})
}
