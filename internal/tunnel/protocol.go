package tunnel

import (
	"net/http"
)

// MessageType define o tipo de mensagem trocada no WebSocket
type MessageType string

const (
	TypeReady        MessageType = "ready"
	TypeHTTPRequest  MessageType = "http_request"
	TypeHTTPResponse MessageType = "http_response"
	TypeAuth         MessageType = "auth"          // CLI → Server: envio do JWT
	TypeAuthRequired MessageType = "auth_required" // Server → CLI: solicita autenticação
	TypeAuthError    MessageType = "auth_error"    // Server → CLI: falha na autenticação
	TypePing         MessageType = "ping"          // Server → CLI: heartbeat
	TypePong         MessageType = "pong"          // CLI → Server: heartbeat response
)

// BaseMessage estrutura base para todas as mensagens
type BaseMessage struct {
	Type MessageType `json:"type"`
}

// ReadyMessage enviada pelo server ao estabelecer o túnel
type ReadyMessage struct {
	BaseMessage
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
}

// AuthMessage enviada pelo CLI ao server com o token JWT
type AuthMessage struct {
	BaseMessage
	Token string `json:"token"`
}

// ErrorMessage enviada pelo server ao CLI quando ocorre um erro
type ErrorMessage struct {
	BaseMessage
	Code    string `json:"code"`    // Código do erro (ex: "AUTH_INVALID", "AUTH_EXPIRED")
	Message string `json:"message"` // Mensagem legível para o usuário
}

// RequestMessage enviada pelo server para o client (proxying)
type RequestMessage struct {
	BaseMessage
	ID      string              `json:"id"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

// ResponseMessage enviada pelo client para o server (proxying)
type ResponseMessage struct {
	BaseMessage
	ID      string              `json:"id"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

// MapHeaders converte http.Header para map serializável
func MapHeaders(h http.Header) map[string][]string {
	m := make(map[string][]string)
	for k, v := range h {
		m[k] = v
	}
	return m
}

