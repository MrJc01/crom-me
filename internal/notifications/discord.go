package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// DiscordWebhook envia notificações para um canal do Discord
func DiscordWebhook(title, description string) error {
	url := os.Getenv("DISCORD_WEBHOOK_URL")
	if url == "" {
		return nil // Silenciosamente ignora se não configurado
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": description,
				"color":       3447003, // Azul Crom
			},
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("erro ao enviar webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord retornou status: %d", resp.StatusCode)
	}

	return nil
}
