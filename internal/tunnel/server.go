package tunnel

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MrJc01/crom-me/internal/auth"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Heartbeat: intervalo de ping e deadline para pong
	pingInterval = 30 * time.Second
	pongWait     = 45 * time.Second

	// Tempo máximo para o CLI enviar o AuthMessage após conectar
	authTimeout = 10 * time.Second
)

type Server struct {
	mu              sync.RWMutex
	connections     map[string]*websocket.Conn
	pendingRequests map[string]chan *ResponseMessage
	upgrader        websocket.Upgrader

	// Metrics
	UserStats   map[uuid.UUID]*atomic.Uint64 // Incrementa a cada requisição HTTP processada
	ActiveUsers map[uuid.UUID]string         // Mapeia userID -> Subdomínio ativo
}

func NewServer() *Server {
	return &Server{
		connections:     make(map[string]*websocket.Conn),
		pendingRequests: make(map[string]chan *ResponseMessage),
		UserStats:       make(map[uuid.UUID]*atomic.Uint64),
		ActiveUsers:     make(map[uuid.UUID]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// HandleClient gerencia a conexão WebSocket de um cliente CLI.
// Fluxo: Upgrade → Solicitar Auth → Validar JWT → Gerar Slug → Heartbeat → Loop de mensagens.
func (s *Server) HandleClient(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// ──────────────────────────────────────────
	// 1. Solicitar autenticação
	// ──────────────────────────────────────────
	authReqMsg := BaseMessage{Type: TypeAuthRequired}
	if err := conn.WriteJSON(authReqMsg); err != nil {
		slog.Error("Falha ao enviar auth_required", "error", err, "remote_addr", r.RemoteAddr)
		return
	}

	// 2. Aguardar AuthMessage com timeout
	conn.SetReadDeadline(time.Now().Add(authTimeout))
	_, p, err := conn.ReadMessage()
	if err != nil {
		slog.Warn("Timeout ou erro aguardando autenticação", "error", err, "remote_addr", r.RemoteAddr)
		conn.WriteJSON(ErrorMessage{
			BaseMessage: BaseMessage{Type: TypeAuthError},
			Code:        "AUTH_TIMEOUT",
			Message:     "Tempo limite para autenticação excedido",
		})
		return
	}

	var authMsg AuthMessage
	if err := json.Unmarshal(p, &authMsg); err != nil || authMsg.Type != TypeAuth {
		conn.WriteJSON(ErrorMessage{
			BaseMessage: BaseMessage{Type: TypeAuthError},
			Code:        "AUTH_INVALID",
			Message:     "Mensagem de autenticação inválida",
		})
		return
	}

	// 3. Validar JWT
	claims, err := auth.ValidateToken(authMsg.Token)
	if err != nil {
		slog.Warn("Token JWT inválido na conexão WebSocket", "error", err, "remote_addr", r.RemoteAddr)
		conn.WriteJSON(ErrorMessage{
			BaseMessage: BaseMessage{Type: TypeAuthError},
			Code:        "AUTH_EXPIRED",
			Message:     "Token inválido ou expirado. Execute: crom-cli auth login",
		})
		return
	}

	slog.Info("Autenticação WebSocket OK", "user_id", claims.UserID, "role", claims.Role, "remote_addr", r.RemoteAddr)

	// Resetar deadline (será gerenciado pelo heartbeat)
	conn.SetReadDeadline(time.Time{})

	// ──────────────────────────────────────────
	// 4. Gerar slug único e registrar conexão
	// ──────────────────────────────────────────
	
	uid, _ := uuid.Parse(claims.UserID)
	
	s.mu.Lock()
	subdomain := GenerateUniqueSlug(s.connections)
	s.connections[subdomain] = conn
	
	// Inicializar contador do usuário se não existir
	if _, exists := s.UserStats[uid]; !exists {
		s.UserStats[uid] = &atomic.Uint64{}
	}
	s.ActiveUsers[uid] = subdomain
	s.mu.Unlock()

	slog.Info("Túnel ativo", "subdomain", subdomain, "user_id", claims.UserID)

	defer func() {
		s.mu.Lock()
		delete(s.connections, subdomain)
		delete(s.ActiveUsers, uid)
		s.mu.Unlock()
		slog.Info("Túnel encerrado", "subdomain", subdomain)
	}()

	// 5. Enviar ReadyMessage ao CLI
	readyMsg := ReadyMessage{
		BaseMessage: BaseMessage{Type: TypeReady},
		Subdomain:   subdomain,
		URL:         fmt.Sprintf("https://%s-free.crom.me", subdomain),
	}
	conn.WriteJSON(readyMsg)

	// ──────────────────────────────────────────
	// 6. Heartbeat (Ping/Pong)
	// ──────────────────────────────────────────
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Goroutine de ping periódico
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// ──────────────────────────────────────────
	// 7. Loop principal de mensagens
	// ──────────────────────────────────────────
	conn.SetReadDeadline(time.Now().Add(pongWait))
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var base BaseMessage
		json.Unmarshal(p, &base)

		switch base.Type {
		case TypeHTTPResponse:
			var resp ResponseMessage
			json.Unmarshal(p, &resp)

			s.mu.RLock()
			ch, ok := s.pendingRequests[resp.ID]
			s.mu.RUnlock()

			if ok {
				ch <- &resp
			}

		case TypePong:
			// Pong recebido — deadline já atualizado pelo PongHandler
			conn.SetReadDeadline(time.Now().Add(pongWait))
		}
	}

	close(done) // Encerra goroutine de ping
}

func (s *Server) HandleTunnelTraffic(w http.ResponseWriter, r *http.Request, subdomain string) {
	s.mu.RLock()
	clientConn, exists := s.connections[subdomain]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Túnel offline", http.StatusNotFound)
		return
	}

	requestID := uuid.New().String()
	respChan := make(chan *ResponseMessage)

	s.mu.Lock()
	s.pendingRequests[requestID] = respChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingRequests, requestID)
		s.mu.Unlock()
	}()

	body, _ := io.ReadAll(r.Body)
	reqMsg := RequestMessage{
		BaseMessage: BaseMessage{Type: TypeHTTPRequest},
		ID:          requestID,
		Method:      r.Method,
		Path:        r.URL.Path,
		Headers:     MapHeaders(r.Header),
		Body:        body,
	}

	s.mu.Lock()
	err := clientConn.WriteJSON(reqMsg)
	s.mu.Unlock()
	
	if err != nil {
		http.Error(w, "Erro ao enviar para o cliente", http.StatusInternalServerError)
		return
	}

	select {
	case resp := <-respChan:
		for k, v := range resp.Headers {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}
		w.WriteHeader(resp.Status)
		w.Write(resp.Body)
		
		// ──────────────────────────────────────────
		// Incrementa métrica global do usuário proxy
		// ──────────────────────────────────────────
		s.mu.RLock()
		for uid, sub := range s.ActiveUsers {
			if sub == subdomain {
				if stat, ok := s.UserStats[uid]; ok {
					stat.Add(1)
				}
				break
			}
		}
		s.mu.RUnlock()

	case <-time.After(30 * time.Second):
		http.Error(w, "Timeout do cliente", http.StatusGatewayTimeout)
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// METRICS EXPORT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// GetUserStats retorna o status atual e total de requsições roteadas pelo túnel local em memória.
func (s *Server) GetUserStats(userID uuid.UUID) (bool, string, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	activeSubdomain, isOnline := s.ActiveUsers[userID]
	
	var requests uint64
	if stat, ok := s.UserStats[userID]; ok {
		requests = stat.Load()
	}
	
	return isOnline, activeSubdomain, requests
}
