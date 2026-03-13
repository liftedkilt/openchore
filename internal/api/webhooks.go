package api

import (
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type WebhookHandler struct {
	store *store.Store
}

func NewWebhookHandler(s *store.Store) *WebhookHandler {
	return &WebhookHandler{store: s}
}

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.store.ListWebhooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}
	if webhooks == nil {
		webhooks = []model.Webhook{}
	}
	writeJSON(w, http.StatusOK, webhooks)
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
		Events string `json:"events"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.Events == "" {
		req.Events = "*"
	}

	wh := &model.Webhook{
		URL:    req.URL,
		Secret: req.Secret,
		Events: req.Events,
		Active: true,
	}
	if err := h.store.CreateWebhook(r.Context(), wh); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	var req struct {
		URL    *string `json:"url"`
		Secret *string `json:"secret"`
		Events *string `json:"events"`
		Active *bool   `json:"active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing webhooks to find this one
	webhooks, err := h.store.ListWebhooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}
	var existing *model.Webhook
	for _, wh := range webhooks {
		if wh.ID == id {
			existing = &wh
			break
		}
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if req.URL != nil {
		existing.URL = *req.URL
	}
	if req.Secret != nil {
		existing.Secret = *req.Secret
	}
	if req.Events != nil {
		existing.Events = *req.Events
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}

	if err := h.store.UpdateWebhook(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}
	if err := h.store.DeleteWebhook(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}
	deliveries, err := h.store.ListWebhookDeliveries(r.Context(), id, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deliveries")
		return
	}
	if deliveries == nil {
		deliveries = []model.WebhookDelivery{}
	}
	writeJSON(w, http.StatusOK, deliveries)
}
