package domain

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MrJc01/crom-me/internal/cloudflare"
	"github.com/MrJc01/crom-me/internal/notifications"
	"github.com/google/uuid"
)

// Status representa o estado de um subdomínio
type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusRejected  Status = "rejected"
)

// Domain representa um subdomínio no sistema
type Domain struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Subdomain   string     `json:"subdomain"`
	Target      string     `json:"target"`
	RecordType  string     `json:"record_type"`
	DNSRecordID *string    `json:"dns_record_id,omitempty"`
	Status      Status     `json:"status"`
	Purpose     string     `json:"purpose"`
	ProjectName string     `json:"project_name"`
	ContactEmail string    `json:"contact_email"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// Quota representa os limites de um usuário
type Quota struct {
	UserID     uuid.UUID `json:"user_id"`
	BaseLimit  int       `json:"base_limit"`
	BonusLimit int       `json:"bonus_limit"`
	UsedSlots  int       `json:"used_slots"`
}

// Repository define a interface para persistência de domínios
type Repository interface {
	Create(ctx context.Context, d *Domain) error
	GetByID(ctx context.Context, id uuid.UUID) (*Domain, error)
	GetBySubdomain(ctx context.Context, subdomain string) (*Domain, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*Domain, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	GetQuota(ctx context.Context, userID uuid.UUID) (*Quota, error)
	IncrementUsedSlots(ctx context.Context, userID uuid.UUID) error
	GetPendingRequests(ctx context.Context) ([]*Domain, error)
	UpdateQuotaLimit(ctx context.Context, userID uuid.UUID, newBaseLimit int) error
	UpdateDNSRecordID(ctx context.Context, domainID uuid.UUID, recordID string) error
	GetAllDomains(ctx context.Context) ([]*Domain, error)
	DecrementUsedSlots(ctx context.Context, userID uuid.UUID) error
}

// Service contém a lógica de negócio para domínios
type Service struct {
	repo     Repository
	cfClient *cloudflare.Client
}

func NewService(repo Repository, cfClient *cloudflare.Client) *Service {
	return &Service{repo: repo, cfClient: cfClient}
}

// RequestSubdomain inicia o processo de solicitação
func (s *Service) RequestSubdomain(ctx context.Context, userID uuid.UUID, subdomain, target, purpose, projectName, contactEmail string) (*Domain, error) {
	quota, err := s.repo.GetQuota(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("falha ao verificar quota: %w", err)
	}

	if quota.UsedSlots >= (quota.BaseLimit + quota.BonusLimit) {
		return nil, fmt.Errorf("limite de subdomínios atingido")
	}

	// Auto-detectar tipo de registro: IP → A, domínio → CNAME
	recordType := "CNAME"
	if net.ParseIP(target) != nil {
		recordType = "A"
	}

	domain := &Domain{
		ID:         uuid.New(),
		UserID:     userID,
		Subdomain:  subdomain,
		Target:     target,
		RecordType:   recordType,
		Status:       StatusPending,
		Purpose:      purpose,
		ProjectName:  projectName,
		ContactEmail: contactEmail,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.Create(ctx, domain); err != nil {
		return nil, fmt.Errorf("erro ao criar solicitação: %w", err)
	}

	// Notifica Admin via Discord (Async)
	go notifications.DiscordWebhook("🆕 Nova Solicitação de Subdomínio", 
		fmt.Sprintf("Usuário: %s\nSubdomínio: **%s.crom.me**\nMotivo: %s", userID, subdomain, purpose))

	return domain, nil
}


// ApproveSubdomain aprova um subdomínio e cria o registro no DNS
func (s *Service) ApproveSubdomain(ctx context.Context, domainID uuid.UUID) error {
	d, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}

	if d.Status != StatusPending {
		return fmt.Errorf("apenas subdomínios pendentes podem ser aprovados")
	}

	// 1. Cria na Cloudflare (DNS Only para rotear para o Target externo em vez de voltar pro Crom)
	cfID, err := s.cfClient.CreateDNSRecord(ctx, cloudflare.DNSRecord{
		Type:    d.RecordType,
		Name:    d.Subdomain + ".crom.me",
		Content: d.Target,
		Proxied: false, // CRÍTICO: Proxied=true roteia pro Nginx da VPS
	})
	if err != nil {
		return fmt.Errorf("falha ao criar registro DNS: %w", err)
	}

	// 2. Salva o dns_record_id no banco para rastreabilidade
	if err := s.repo.UpdateDNSRecordID(ctx, domainID, cfID); err != nil {
		return fmt.Errorf("falha ao salvar dns_record_id: %w", err)
	}

	// 3. Atualiza status para ativo
	if err := s.repo.UpdateStatus(ctx, domainID, StatusActive); err != nil {
		return err
	}

	// 4. Incrementa quota usada
	if err := s.repo.IncrementUsedSlots(ctx, d.UserID); err != nil {
		return err
	}

	// 5. Notifica via Discord
	go notifications.DiscordWebhook("✅ Subdomínio Aprovado",
		fmt.Sprintf("Subdomínio: **%s.crom.me**\nAlvo: %s\nCloudflare ID: %s", d.Subdomain, d.Target, cfID))

	return nil
}

// RejectSubdomain rejeita uma solicitação e deleta o registro para liberar o subdomínio
func (s *Service) RejectSubdomain(ctx context.Context, domainID uuid.UUID) error {
	d, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}

	// Deleta o registro ao invés de marcar como rejected,
	// liberando a constraint UNIQUE para re-solicitação
	if err := s.repo.DeleteByID(ctx, domainID); err != nil {
		return err
	}

	// Notifica via Discord
	go notifications.DiscordWebhook("❌ Subdomínio Rejeitado",
		fmt.Sprintf("Subdomínio: **%s.crom.me**\nMotivo: Curadoria administrativa", d.Subdomain))

	return nil
}

// ListPendingRequests retorna todas as solicitações aguardando aprovação
func (s *Service) ListPendingRequests(ctx context.Context) ([]*Domain, error) {
	return s.repo.GetPendingRequests(ctx)
}

// GetUserSubdomains retorna os subdomínios de um usuário específico
func (s *Service) GetUserSubdomains(ctx context.Context, userID uuid.UUID) ([]*Domain, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// GetUserQuota retorna a quota atual de um usuário
func (s *Service) GetUserQuota(ctx context.Context, userID uuid.UUID) (*Quota, error) {
	return s.repo.GetQuota(ctx, userID)
}

// PromoteUser aumenta o base_limit de slots de um usuário (promoção a parceiro)
func (s *Service) PromoteUser(ctx context.Context, userID uuid.UUID, newBaseLimit int) error {
	if newBaseLimit < 1 || newBaseLimit > 1000 {
		return fmt.Errorf("base_limit deve estar entre 1 e 1000")
	}

	if err := s.repo.UpdateQuotaLimit(ctx, userID, newBaseLimit); err != nil {
		return err
	}

	// Notifica via Discord
	go notifications.DiscordWebhook("🚀 Usuário Promovido",
		fmt.Sprintf("UserID: %s\nNovo limite: **%d slots**", userID, newBaseLimit))

	return nil
}

// RemoveDomain remove um domínio ativo, deletando o registro DNS na Cloudflare e liberando a quota
func (s *Service) RemoveDomain(ctx context.Context, domainID uuid.UUID) error {
	d, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}

	// Remove do Cloudflare se tiver DNS Record ID
	if d.DNSRecordID != nil && *d.DNSRecordID != "" {
		if err := s.cfClient.DeleteDNSRecord(ctx, *d.DNSRecordID); err != nil {
			// Log mas não bloqueia a remoção do banco
			fmt.Printf("WARN: falha ao deletar registro DNS %s: %v\n", *d.DNSRecordID, err)
		}
	}

	// Deleta do banco
	if err := s.repo.DeleteByID(ctx, domainID); err != nil {
		return err
	}

	// Decrementa quota se estava ativo
	if d.Status == StatusActive {
		s.repo.DecrementUsedSlots(ctx, d.UserID)
	}

	go notifications.DiscordWebhook("🗑️ Domínio Removido (Admin)",
		fmt.Sprintf("Subdomínio: **%s.crom.me**\nAlvo: %s", d.Subdomain, d.Target))

	return nil
}

// AdminCreateDomain cria um domínio direto sem fila de aprovação (Admin bypass)
func (s *Service) AdminCreateDomain(ctx context.Context, subdomain, target, purpose, projectName, contactEmail string) (*Domain, error) {
	// Auto-detectar tipo de registro
	recordType := "CNAME"
	if net.ParseIP(target) != nil {
		recordType = "A"
	}

	domain := &Domain{
		ID:           uuid.New(),
		UserID:       uuid.Nil, // Admin-owned
		Subdomain:    subdomain,
		Target:       target,
		RecordType:   recordType,
		Status:       StatusActive,
		Purpose:      purpose,
		ProjectName:  projectName,
		ContactEmail: contactEmail,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Cria na Cloudflare
	cfID, err := s.cfClient.CreateDNSRecord(ctx, cloudflare.DNSRecord{
		Type:    recordType,
		Name:    subdomain + ".crom.me",
		Content: target,
		Proxied: true,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao criar registro DNS: %w", err)
	}

	domain.DNSRecordID = &cfID

	if err := s.repo.Create(ctx, domain); err != nil {
		return nil, fmt.Errorf("erro ao salvar domínio: %w", err)
	}

	go notifications.DiscordWebhook("⚡ Domínio Criado (Admin Direct)",
		fmt.Sprintf("Subdomínio: **%s.crom.me** → %s", subdomain, target))

	return domain, nil
}

// ListAllDomains retorna todos os domínios cadastrados (Admin)
func (s *Service) ListAllDomains(ctx context.Context) ([]*Domain, error) {
	return s.repo.GetAllDomains(ctx)
}
