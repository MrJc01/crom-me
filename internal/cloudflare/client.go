package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client encapsula a comunicação com a API v4 da Cloudflare
type Client struct {
	apiToken string
	zoneID   string
	baseURL  string
	HTTP     *http.Client
}

// DNSRecord representa um registro DNS na Cloudflare
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl,omitempty"`
	Proxied bool   `json:"proxied"`
}

// APIResponse estrutura básica de resposta da Cloudflare
type APIResponse struct {
	Success  bool            `json:"success"`
	Errors   []interface{}   `json:"errors"`
	Messages []interface{}   `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

// NewClient cria uma nova instância do cliente Cloudflare
func NewClient(apiToken, zoneID string) *Client {
	return &Client{
		apiToken: apiToken,
		zoneID:   zoneID,
		baseURL:  "https://api.cloudflare.com/client/v4",
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateDNSRecord cria um novo registro DNS (A, CNAME, etc)
func (c *Client) CreateDNSRecord(ctx context.Context, record DNSRecord) (string, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, c.zoneID)
	
	body, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("erro ao codificar registro: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha na requisição: %w", err)
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if !apiResp.Success {
		return "", fmt.Errorf("erro da API Cloudflare: %v", apiResp.Errors)
	}

	var result DNSRecord
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return "", fmt.Errorf("erro ao parsear resultado: %w", err)
	}

	return result.ID, nil
}

// DeleteDNSRecord remove um registro DNS pelo ID
func (c *Client) DeleteDNSRecord(ctx context.Context, recordID string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, c.zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("falha na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status inesperado (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
}
