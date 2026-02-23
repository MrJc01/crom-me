package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/MrJc01/crom-me/internal/api"
	"github.com/MrJc01/crom-me/internal/auth"
	"github.com/MrJc01/crom-me/internal/cloudflare"
	"github.com/MrJc01/crom-me/internal/database"
	"github.com/MrJc01/crom-me/internal/domain"
	"github.com/MrJc01/crom-me/internal/tunnel"
)

func main() {
	ctx := context.Background()

	// 0. Configurar Logger Global
	logLevel := slog.LevelInfo
	if os.Getenv("APP_ENV") == "development" {
		logLevel = slog.LevelDebug
	}

	var handler slog.Handler
	if os.Getenv("APP_ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// 1. Banco de Dados
	db, err := database.NewDB(ctx)
	if err != nil {
		slog.Error("Falha ao conectar no banco", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 2. Repositories & Services
	repo := database.NewPostgresRepository(db)
	
	// Cloudflare (Carrega das envs)
	cfClient := cloudflare.NewClient(
		os.Getenv("CLOUDFLARE_API_TOKEN"),
		os.Getenv("CLOUDFLARE_ZONE_ID"),
	)

	// Instanciar WebSocket Tunnel primeiro para permitir injeção cruzada
	tunnelSvc := tunnel.NewServer()

	domainSvc := domain.NewService(repo, cfClient)
	apiHandler := api.NewHandler(domainSvc, tunnelSvc)

	// Auth (GitHub OAuth)
	oauthProvider := auth.NewGitHubProvider()
	authHandler := api.NewAuthHandler(oauthProvider, repo)


	// Carrega porta
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 3. Middlewares e Segurança
	// Limite: 5 requisições por segundo por IP, com burst de 10
	limiter := api.NewIPRateLimiter(5, 10)
	apiWithRateLimit := func(h http.HandlerFunc) http.Handler {
		return api.RateLimitMiddleware(limiter, h)
	}

	// 4. Mux da API Principal
	apiMux := http.NewServeMux()

	// Servir arquivos estáticos
	fs := http.FileServer(http.Dir("web/static"))
	apiMux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Rota da Landing Page
	apiMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "web/templates/index.html")
	})

	// Auth Routes
	apiMux.HandleFunc("/auth/login", authHandler.HandleLogin)
	apiMux.HandleFunc("/auth/callback", authHandler.HandleCallback)
	apiMux.HandleFunc("/auth/logout", authHandler.HandleLogout)
	apiMux.HandleFunc("/auth/me", api.AuthMiddleware(authHandler.HandleMe))

	// Dashboard e Subdomínios (Protegidos com Auth + Rate Limit)
	apiMux.HandleFunc("/dashboard", api.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/dashboard.html")
	}))
	apiMux.Handle("/api/subdomains/request", apiWithRateLimit(api.AuthMiddleware(apiHandler.RequestSubdomainHandler)))
	apiMux.HandleFunc("/api/user/subdomains", api.AuthMiddleware(apiHandler.UserSubdomainsHandler))
	apiMux.HandleFunc("/api/user/quota", api.AuthMiddleware(apiHandler.UserQuotaHandler))
	apiMux.HandleFunc("/api/user/stats", api.AuthMiddleware(apiHandler.UserStatsHandler))

	// Admin (Protegido com AdminMiddleware)
	apiMux.HandleFunc("/admin", api.AdminMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/admin.html")
	}))
	apiMux.HandleFunc("/api/admin/pending", api.AdminMiddleware(apiHandler.ListPendingHandler))
	apiMux.HandleFunc("/api/admin/approve", api.AdminMiddleware(apiHandler.ApproveHandler))
	apiMux.HandleFunc("/api/admin/reject", api.AdminMiddleware(apiHandler.RejectHandler))
	apiMux.HandleFunc("/api/admin/promote", api.AdminMiddleware(apiHandler.PromoteHandler))
	apiMux.HandleFunc("/api/admin/users", api.AdminMiddleware(authHandler.ListUsersHandler))
	apiMux.HandleFunc("/api/admin/ban", api.AdminMiddleware(authHandler.BanUserHandler))
	apiMux.HandleFunc("/api/admin/unban", api.AdminMiddleware(authHandler.UnbanUserHandler))
	apiMux.HandleFunc("/api/admin/domains", api.AdminMiddleware(apiHandler.ListAllDomainsHandler))
	apiMux.HandleFunc("/api/admin/domains/create", api.AdminMiddleware(apiHandler.AdminCreateDomainHandler))
	apiMux.HandleFunc("/api/admin/domains/delete", api.AdminMiddleware(apiHandler.DeleteDomainHandler))

	// Tunnel WS genérico
	apiMux.Handle("/ws/tunnel", apiWithRateLimit(tunnelSvc.HandleClient))

	// 5. Roteador Principal (Host Routing)
	mainRouter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0] // Remove porta se houver (ex: localhost:8080 -> localhost)

		// 1. Se for o domínio raiz ou servidor local, roteia para a API
		if host == "crom.me" || host == "localhost" || host == "127.0.0.1" {
			apiMux.ServeHTTP(w, r)
			return
		}

		// 2. Se for um subdomínio (ex: tigre-agil.crom.me ou google.crom.me)
		if strings.HasSuffix(host, ".crom.me") {
			subdomain := strings.TrimSuffix(host, ".crom.me")
			// Opcional: lidar com free.crom.me (ex: tigre-agil.free.crom.me)
			if strings.HasSuffix(subdomain, ".free") {
				subdomain = strings.TrimSuffix(subdomain, ".free")
			}
			
			// 2.A: Verifica se é um domínio ESTÁTICO cadastrado e APROVADO via Painel (Banco de Dados)
			// Buscamos ignorando o usuário (precisaria ser um repositório GetDomainByName)
			// Como o GetDomainByName não existe no Service atual, injetamos pelo db diretamente ou criamos.
			// Para uma validação imediata e performática, vamos ler a lista de domínios ativos em cache ou db.
			domainInfo, err := repo.GetBySubdomain(r.Context(), subdomain)
			if err == nil && domainInfo != nil && domainInfo.Status == domain.StatusActive {
				// Proxy Reverso para o Target externo
				targetURL, err := url.Parse(domainInfo.Target)
				if err == nil {
					// Se o Target for só IP "148.230.79.173", o ParseURL fica vazio no scheme, então fix:
					if targetURL.Scheme == "" {
						targetURL, _ = url.Parse("http://" + domainInfo.Target)
					}
					
					proxy := httputil.NewSingleHostReverseProxy(targetURL)
					
					// Modificar headers para não quebrar a origem no Cloudflare backend do dev
					r.URL.Host = targetURL.Host
					r.URL.Scheme = targetURL.Scheme
					r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
					r.Host = targetURL.Host
					
					proxy.ServeHTTP(w, r)
					return
				}
			}

			// 2.B: Se não é domínio Estático Aprovado, assume que é TÚNEL CLI DINÂMICO via WebSocket
			tunnelSvc.HandleTunnelTraffic(w, r, subdomain)
			return
		}

		// 3. Fallback: roteia para API (se não era .crom.me, ou se caiu por algum motivo obscuro)
		apiMux.ServeHTTP(w, r)
	})

	slog.Info("Servidor crom.me iniciado", "port", port, "env", os.Getenv("APP_ENV"))
	if err := http.ListenAndServe(":"+port, mainRouter); err != nil {
		slog.Error("Erro fatal no servidor", "error", err)
		os.Exit(1)
	}
}
