package api

import (
	"encoding/json"
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type PointsHandler struct {
	store *store.Store
}

func NewPointsHandler(s *store.Store) *PointsHandler {
	return &PointsHandler{store: s}
}

func (h *PointsHandler) GetUserPoints(w http.ResponseWriter, r *http.Request) {
	userID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	balance, err := h.store.GetPointBalance(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get balance")
		return
	}
	txs, err := h.store.ListPointTransactions(r.Context(), userID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get transactions")
		return
	}
	if txs == nil {
		txs = []model.PointTransaction{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"balance":      balance,
		"transactions": txs,
	})
}

func (h *PointsHandler) GetAllBalances(w http.ResponseWriter, r *http.Request) {
	balances, err := h.store.GetAllPointBalances(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get balances")
		return
	}
	writeJSON(w, http.StatusOK, balances)
}

func (h *PointsHandler) Adjust(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int64  `json:"user_id"`
		Amount int    `json:"amount"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == 0 || req.Amount == 0 {
		writeError(w, http.StatusBadRequest, "user_id and non-zero amount required")
		return
	}
	if err := h.store.AdminAdjustPoints(r.Context(), req.UserID, req.Amount, req.Note); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to adjust points")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PointsHandler) GetDecayConfig(w http.ResponseWriter, r *http.Request) {
	userID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	cfg, err := h.store.GetUserDecayConfig(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get decay config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *PointsHandler) SetDecayConfig(w http.ResponseWriter, r *http.Request) {
	userID, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Enabled            bool `json:"enabled"`
		DecayRate          int  `json:"decay_rate"`
		DecayIntervalHours int  `json:"decay_interval_hours"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DecayRate < 0 {
		writeError(w, http.StatusBadRequest, "decay_rate must be non-negative")
		return
	}
	if req.DecayIntervalHours <= 0 {
		req.DecayIntervalHours = 24
	}
	cfg := &model.UserDecayConfig{
		UserID:             userID,
		Enabled:            req.Enabled,
		DecayRate:          req.DecayRate,
		DecayIntervalHours: req.DecayIntervalHours,
	}
	if err := h.store.SetUserDecayConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update decay config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
