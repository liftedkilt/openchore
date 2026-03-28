package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// Settings key for the Discord webhook URL in app_settings.
const SettingWebhookURL = "discord_webhook_url"

// Embed colors
const (
	ColorGreen  = 0x22c55e // completions approved
	ColorYellow = 0xeab308 // pending approval
	ColorRed    = 0xef4444 // rejected
)

// Notifier sends formatted messages to a Discord webhook.
type Notifier struct {
	store  *store.Store
	client *http.Client
}

// NewNotifier creates a Discord notifier.
func NewNotifier(s *store.Store) *Notifier {
	return &Notifier{
		store: s,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// discordEmbed represents a Discord embed object.
type discordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Thumbnail   *discordImage  `json:"thumbnail,omitempty"`
	Fields      []discordField `json:"fields,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

type discordImage struct {
	URL string `json:"url"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

// NotifyPendingApproval sends a notification when a chore completion is submitted and requires approval.
func (n *Notifier) NotifyPendingApproval(userName, choreTitle, photoURL string) {
	embed := discordEmbed{
		Title:       "Chore Submitted for Approval",
		Description: fmt.Sprintf("**%s** completed **%s** — awaiting approval", userName, choreTitle),
		Color:       ColorYellow,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []discordField{
			{Name: "Kid", Value: userName, Inline: true},
			{Name: "Chore", Value: choreTitle, Inline: true},
		},
	}
	if photoURL != "" {
		embed.Thumbnail = &discordImage{URL: photoURL}
	}
	go n.send(embed)
}

// NotifyApproved sends a notification when a chore completion is approved.
func (n *Notifier) NotifyApproved(userName, choreTitle string) {
	embed := discordEmbed{
		Title:       "Chore Approved",
		Description: fmt.Sprintf("**%s**'s completion of **%s** was approved", userName, choreTitle),
		Color:       ColorGreen,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []discordField{
			{Name: "Kid", Value: userName, Inline: true},
			{Name: "Chore", Value: choreTitle, Inline: true},
		},
	}
	go n.send(embed)
}

// NotifyRejected sends a notification when a chore completion is rejected.
func (n *Notifier) NotifyRejected(userName, choreTitle string) {
	embed := discordEmbed{
		Title:       "Chore Rejected",
		Description: fmt.Sprintf("**%s**'s completion of **%s** was rejected", userName, choreTitle),
		Color:       ColorRed,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []discordField{
			{Name: "Kid", Value: userName, Inline: true},
			{Name: "Chore", Value: choreTitle, Inline: true},
		},
	}
	go n.send(embed)
}

// NotifyCompleted sends a notification when a chore is completed (auto-approved).
func (n *Notifier) NotifyCompleted(userName, choreTitle, photoURL string, pointsEarned int) {
	desc := fmt.Sprintf("**%s** completed **%s**", userName, choreTitle)
	if pointsEarned > 0 {
		desc += fmt.Sprintf(" (+%d pts)", pointsEarned)
	}
	embed := discordEmbed{
		Title:       "Chore Completed",
		Description: desc,
		Color:       ColorGreen,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []discordField{
			{Name: "Kid", Value: userName, Inline: true},
			{Name: "Chore", Value: choreTitle, Inline: true},
		},
	}
	if photoURL != "" {
		embed.Thumbnail = &discordImage{URL: photoURL}
	}
	go n.send(embed)
}

// send posts the embed to the configured Discord webhook URL.
// It is non-blocking (called via goroutine) and silently no-ops if no URL is configured.
func (n *Notifier) send(embed discordEmbed) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	webhookURL, err := n.store.GetSetting(ctx, SettingWebhookURL)
	if err != nil || webhookURL == "" {
		return // No Discord webhook configured; silently skip.
	}

	payload := discordPayload{Embeds: []discordEmbed{embed}}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("discord: failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("discord: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		log.Printf("discord: failed to send webhook: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("discord: webhook returned status %d", resp.StatusCode)
	}
}
