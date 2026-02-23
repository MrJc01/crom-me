package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MrJc01/crom-me/internal/auth"
	"github.com/MrJc01/crom-me/internal/domain"
	"github.com/MrJc01/crom-me/internal/tunnel"
	"github.com/google/uuid"
)

type Handler struct {
	domainSvc *domain.Service
	tunnelSvc *tunnel.Server
}

func NewHandler(svc *domain.Service, tunnelSrv *tunnel.Server) *Handler {
	return &Handler{
		domainSvc: svc,
		tunnelSvc: tunnelSrv,
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// USER ENDPOINTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// RequestSubdomainHandler lida com a solicitação de novos subdomínios
func (h *Handler) RequestSubdomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Recuperar UserID do contexto JWT
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	var req struct {
		Subdomain    string `json:"subdomain"`
		Target       string `json:"target"`
		Purpose      string `json:"purpose"`
		ProjectName  string `json:"project_name"`
		ContactEmail string `json:"contact_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	d, err := h.domainSvc.RequestSubdomain(r.Context(), userID, req.Subdomain, req.Target, req.Purpose, req.ProjectName, req.ContactEmail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

// UserSubdomainsHandler retorna os subdomínios do usuário logado
func (h *Handler) UserSubdomainsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	domains, err := h.domainSvc.GetUserSubdomains(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Retorna array vazio ao invés de null se não houver domínios
	if domains == nil {
		domains = []*domain.Domain{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// UserQuotaHandler retorna a quota do usuário logado
func (h *Handler) UserQuotaHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	quota, err := h.domainSvc.GetUserQuota(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quota)
}

// UserStatsHandler retorna o status vivo e tráfego proxyado do WebSocket (CLI)
func (h *Handler) UserStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	isOnline, activeSub, requests := h.tunnelSvc.GetUserStats(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_online":        isOnline,
		"active_subdomain": activeSub,
		"routed_requests":  requests,
	})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ADMIN ENDPOINTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ListPendingHandler lista todas as solicitações pendentes (Admin)
func (h *Handler) ListPendingHandler(w http.ResponseWriter, r *http.Request) {
	domains, err := h.domainSvc.ListPendingRequests(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if domains == nil {
		domains = []*domain.Domain{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// ApproveHandler aprova uma solicitação (Admin)
func (h *Handler) ApproveHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := h.domainSvc.ApproveSubdomain(r.Context(), id); err != nil {
		slog.Error("Erro ao aprovar subdomínio", "error", err, "domain_id", idStr)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "approved", "message": "Aprovado com sucesso"})
}

// RejectHandler rejeita uma solicitação (Admin)
func (h *Handler) RejectHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := h.domainSvc.RejectSubdomain(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected", "message": "Rejeitado com sucesso"})
}

// PromoteHandler promove um usuário aumentando seu base_limit (Admin)
func (h *Handler) PromoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID    string `json:"user_id"`
		BaseLimit int    `json:"base_limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "user_id inválido", http.StatusBadRequest)
		return
	}

	if err := h.domainSvc.PromoteUser(r.Context(), userID, req.BaseLimit); err != nil {
		slog.Error("Erro ao promover usuário", "error", err, "target_user_id", req.UserID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("Usuário promovido com sucesso", "target_user_id", req.UserID, "new_base_limit", req.BaseLimit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "promoted",
		"user_id":    req.UserID,
		"base_limit": req.BaseLimit,
	})
}

// AdminCreateDomainHandler permite ao Admin criar um domínio ignorando a fila
func (h *Handler) AdminCreateDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Subdomain    string `json:"subdomain"`
		Target       string `json:"target"`
		Purpose      string `json:"purpose"`
		ProjectName  string `json:"project_name"`
		ContactEmail string `json:"contact_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	d, err := h.domainSvc.AdminCreateDomain(r.Context(), req.Subdomain, req.Target, req.Purpose, req.ProjectName, req.ContactEmail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

// DeleteDomainHandler remove um domínio completamente do sistema
func (h *Handler) DeleteDomainHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := h.domainSvc.RemoveDomain(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "message": "Domínio removido com sucesso"})
}

// ListAllDomainsHandler retorna todos os domínios para a tabela do Admin
func (h *Handler) ListAllDomainsHandler(w http.ResponseWriter, r *http.Request) {
	domains, err := h.domainSvc.ListAllDomains(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if domains == nil {
		domains = []*domain.Domain{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// MIDDLEWARE
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// AuthMiddleware protege as rotas e injeta os claims do JWT no contexto
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		claims, err := auth.ValidateToken(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Injeta claims no contexto usando chave tipada
		ctx := context.WithValue(r.Context(), domain.ContextKeyClaims, claims)
		next(w, r.WithContext(ctx))
	}
}

// AdminMiddleware protege rotas administrativas validando role == "admin"
func AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaimsFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			http.Error(w, "Acesso negado: permissão de administrador necessária", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// GetClaimsFromContext extrai os claims do JWT do contexto da requisição
func GetClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, ok := ctx.Value(domain.ContextKeyClaims).(*auth.Claims)
	if !ok {
		return nil
	}
	return claims
}

// getUserIDFromContext extrai e parseia o UserID dos claims do contexto
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims := GetClaimsFromContext(ctx)
	if claims == nil {
		return uuid.Nil, fmt.Errorf("claims não encontrados no contexto")
	}
	return uuid.Parse(claims.UserID)
}
