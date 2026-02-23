package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/MrJc01/crom-me/internal/tunnel"
	"github.com/gorilla/websocket"
)

// Config armazena a configuração local do CLI
type Config struct {
	Token     string `json:"token"`
	ServerURL string `json:"server_url"`
}

const defaultServerURL = "wss://crom.me/ws/tunnel"
const defaultAuthURL = "https://crom.me/auth/login"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "auth":
		if len(os.Args) < 3 || os.Args[2] != "login" {
			fmt.Println("Uso: crom-cli auth login")
			os.Exit(1)
		}
		handleAuthLogin()

	case "tunnel", "-port":
		// Compatibilidade com o formato antigo: crom-cli -port 3000
		port := 0
		if os.Args[1] == "-port" && len(os.Args) >= 3 {
			fmt.Sscanf(os.Args[2], "%d", &port)
		} else if len(os.Args) >= 3 {
			fmt.Sscanf(os.Args[2], "%d", &port)
		}
		if port == 0 {
			fmt.Println("Uso: crom-cli tunnel <porta>")
			fmt.Println("  Ex: crom-cli tunnel 3000")
			os.Exit(1)
		}
		handleTunnel(port)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`
╔════════════════════════════════════════════════════╗
║  crom-cli v0.2.0 — Túnel seguro para devs        ║
╠════════════════════════════════════════════════════╣
║                                                    ║
║  Comandos:                                         ║
║    auth login    Autenticar via GitHub              ║
║    tunnel <port> Expor porta local via túnel        ║
║                                                    ║
║  Exemplos:                                         ║
║    crom-cli auth login                             ║
║    crom-cli tunnel 3000                            ║
║    crom-cli -port 3000  (compatível)               ║
║                                                    ║
╚════════════════════════════════════════════════════╝`)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// AUTH LOGIN — Fluxo via browser (padrão gh auth)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleAuthLogin() {
	fmt.Println("🔑 Iniciando autenticação via GitHub...")

	// 1. Iniciar mini HTTP server em porta aleatória
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("❌ Erro ao iniciar servidor local: %v", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", localPort)

	fmt.Printf("   Aguardando callback em %s\n", callbackURL)

	// Canal para receber o token
	tokenChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// 2. Handler do callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Token não recebido", http.StatusBadRequest)
			errChan <- fmt.Errorf("callback sem token")
			return
		}

		// Responder ao browser com sucesso
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>crom.me — Autenticado</title></head>
			<body style="background:#0a0a0a;color:#00ff88;font-family:monospace;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;">
				<div style="text-align:center;">
					<h1>✅ Autenticado com sucesso!</h1>
					<p>Pode fechar esta aba e voltar ao terminal.</p>
				</div>
			</body>
			</html>
		`))

		tokenChan <- token
	})

	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 3. Abrir browser com a URL de login
	loginURL := fmt.Sprintf("%s?cli_redirect=%s", defaultAuthURL, callbackURL)
	fmt.Printf("   Abrindo browser: %s\n", loginURL)

	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("\n⚠️  Não foi possível abrir o browser automaticamente.\n")
		fmt.Printf("   Abra manualmente: %s\n\n", loginURL)
	}

	// 4. Aguardar token (timeout de 2 minutos)
	fmt.Println("   Aguardando autorização no browser...")

	select {
	case token := <-tokenChan:
		// 5. Salvar token no config
		if err := saveConfig(Config{Token: token, ServerURL: defaultServerURL}); err != nil {
			log.Fatalf("❌ Erro ao salvar configuração: %v", err)
		}

		configPath := getConfigPath()
		fmt.Printf("\n✅ Token salvo em: %s\n", configPath)
		fmt.Println("   Agora use: crom-cli tunnel <porta>")

	case err := <-errChan:
		log.Fatalf("❌ Erro durante autenticação: %v", err)

	case <-time.After(2 * time.Minute):
		log.Fatal("❌ Timeout: autenticação não concluída em 2 minutos")
	}

	// Shutdown do mini server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TUNNEL — Conexão WebSocket autenticada
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleTunnel(port int) {
	// 1. Carregar configuração
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("❌ Não autenticado. Execute primeiro: crom-cli auth login")
		os.Exit(1)
	}

	if cfg.Token == "" {
		fmt.Println("❌ Token vazio. Execute: crom-cli auth login")
		os.Exit(1)
	}

	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	// 2. Conectar ao servidor via WebSocket
	fmt.Printf("🔌 Conectando ao servidor %s...\n", serverURL)

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatalf("❌ Erro ao conectar ao servidor de túnel: %v", err)
	}
	defer conn.Close()

	// 3. Aguardar auth_required e enviar token
	_, p, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("❌ Erro ao ler mensagem do servidor: %v", err)
	}

	var base tunnel.BaseMessage
	json.Unmarshal(p, &base)

	if base.Type == tunnel.TypeAuthRequired {
		// Enviar AuthMessage com o JWT
		authMsg := tunnel.AuthMessage{
			BaseMessage: tunnel.BaseMessage{Type: tunnel.TypeAuth},
			Token:       cfg.Token,
		}
		if err := conn.WriteJSON(authMsg); err != nil {
			log.Fatalf("❌ Erro ao enviar autenticação: %v", err)
		}
	}

	// 3.5. Mutex para escrita segura no websocket (previnir Panic de chamadas de goroutines concorrentes)
	mu := &sync.Mutex{}

	// 4. Configurar signal handler
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// 5. Loop de mensagens
	go func() {
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("🔌 Conexão encerrada: %v", err)
				stop <- syscall.SIGTERM
				return
			}

			var base tunnel.BaseMessage
			json.Unmarshal(p, &base)

			switch base.Type {
			case tunnel.TypeReady:
				var msg tunnel.ReadyMessage
				json.Unmarshal(p, &msg)
				fmt.Printf("\n╔════════════════════════════════════════════════════╗\n")
				fmt.Printf("║  crom-cli tunnel v0.2.0                            ║\n")
				fmt.Printf("║                                                    ║\n")
				fmt.Printf("║  Status:    🟢 ONLINE                              ║\n")
				fmt.Printf("║  Slug:      %-37s ║\n", msg.Subdomain)
				fmt.Printf("║  Forwarding: %-37s ║\n", msg.URL)
				fmt.Printf("║           → http://localhost:%-21d ║\n", port)
				fmt.Printf("╚════════════════════════════════════════════════════╝\n\n")

			case tunnel.TypeAuthError:
				var msg tunnel.ErrorMessage
				json.Unmarshal(p, &msg)
				fmt.Printf("\n❌ Falha na autenticação: [%s] %s\n", msg.Code, msg.Message)
				fmt.Println("   Execute: crom-cli auth login")
				os.Exit(1)

			case tunnel.TypeHTTPRequest:
				var msg tunnel.RequestMessage
				json.Unmarshal(p, &msg)
				log.Printf("← %s %s", msg.Method, msg.Path)
				go handleRequest(conn, msg, port, mu)
			}
		}
	}()

	<-stop
	fmt.Println("\n👋 Encerrando túnel...")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleRequest(conn *websocket.Conn, reqMsg tunnel.RequestMessage, localPort int, mu *sync.Mutex) {
	url := fmt.Sprintf("http://localhost:%d%s", localPort, reqMsg.Path)

	req, err := http.NewRequest(reqMsg.Method, url, bytes.NewBuffer(reqMsg.Body))
	if err != nil {
		return
	}

	for k, v := range reqMsg.Headers {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ Erro ao fazer requisição local (%s): %v", reqMsg.Path, err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	respMsg := tunnel.ResponseMessage{
		BaseMessage: tunnel.BaseMessage{Type: tunnel.TypeHTTPResponse},
		ID:          reqMsg.ID,
		Status:      resp.StatusCode,
		Headers:     tunnel.MapHeaders(resp.Header),
		Body:        body,
	}

	log.Printf("→ %d %s (%d bytes)", resp.StatusCode, reqMsg.Path, len(body))
	
	mu.Lock()
	conn.WriteJSON(respMsg)
	mu.Unlock()
}

// getConfigPath retorna o caminho do arquivo de configuração
func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "crom", "config.json")
}

// saveConfig salva a configuração no disco
func saveConfig(cfg Config) error {
	path := getConfigPath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("erro ao criar diretório de config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("erro ao salvar config: %w", err)
	}

	return nil
}

// loadConfig carrega a configuração do disco
func loadConfig() (*Config, error) {
	path := getConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config não encontrada: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config inválida: %w", err)
	}

	return &cfg, nil
}

// openBrowser abre uma URL no navegador padrão do sistema
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("plataforma não suportada: %s", runtime.GOOS)
	}

	return cmd.Start()
}
