package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	discordWebhook string
	client         *http.Client
}

func New(discordWebhook string) *Notifier {
	if discordWebhook == "" {
		return nil
	}
	return &Notifier{
		discordWebhook: discordWebhook,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *Notifier) Notify(domain string) {
	if n == nil || n.discordWebhook == "" || domain == "" {
		return
	}

	payload := map[string]interface{}{
		"content": fmt.Sprintf("🎯 **Heimdall** spotted: `%s`", domain),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", n.discordWebhook, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
