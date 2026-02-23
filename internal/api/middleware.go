package api

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter gerencia limitadores por endereço IP
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.Mutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware limita requisições por IP
func RateLimitMiddleware(limiter *IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr // Em produção, usar header X-Forwarded-For se atrás de proxy
		
		if !limiter.GetLimiter(ip).Allow() {
			http.Error(w, "Muitas requisições. Tente novamente mais tarde.", http.StatusTooManyRequests)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Cleanup ativa uma rotina para limpar IPs antigos (opcional para MVP robusto)
func (i *IPRateLimiter) Cleanup(interval time.Duration) {
	for {
		time.Sleep(interval)
		i.mu.Lock()
		// Simples: limpa tudo para evitar vazamento de memória se o tráfego for massivo
		// Em produção, usaríamos uma lógica de LastSeen
		i.ips = make(map[string]*rate.Limiter)
		i.mu.Unlock()
	}
}
