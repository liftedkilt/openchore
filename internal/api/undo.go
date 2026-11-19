package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// UndoPointsEntry undoes one row of a person's points history. Whatever it
// takes to undo it, the entry then drops out of the history (see
// store.ListVisiblePointTransactions), so there are no "Undo:" rows:
//
//   - chore points: the chore is unchecked, which takes the points (and any
//     auto-save) back the same way unchecking it on the chore list does;
//   - reward purchase: the redemption is refunded and its row removed;
//   - penalty, manual adjustment, streak bonus, goal saving: a linked
//     reversal row cancels it (store.ReversePointTransaction).
//
// Rows that are themselves an undo (chore unchecked, savings returned) can't
// be undone; check the chore again instead.
func (h *ChoreHandler) UndoPointsEntry(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
	txn, err := h.store.GetPointTransaction(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load transaction")
		return
	}

	switch txn.Reason {
	case model.ReasonChoreComplete:
		if txn.ReferenceID == nil {
			writeError(w, http.StatusUnprocessableEntity, store.ErrCannotUndo.Error())
			return
		}
		cc, err := h.store.GetCompletion(r.Context(), *txn.ReferenceID)
		if err != nil || cc == nil {
			writeError(w, http.StatusUnprocessableEntity, "that chore no longer exists")
			return
		}
		if cc.UncompletedAt != nil {
			writeError(w, http.StatusConflict, store.ErrAlreadyUndone.Error())
			return
		}
		if status, msg := h.uncomplete(r, cc.ChoreScheduleID, cc.CompletionDate); status != 0 {
			writeError(w, status, msg)
			return
		}

	case model.ReasonRewardRedeem:
		if txn.ReferenceID == nil {
			writeError(w, http.StatusUnprocessableEntity, store.ErrCannotUndo.Error())
			return
		}
		if err := h.store.UndoRedemption(r.Context(), *txn.ReferenceID); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

	default:
		_, err := h.store.ReversePointTransaction(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrCannotUndo):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		case errors.Is(err, store.ErrAlreadyUndone), errors.Is(err, store.ErrSavingsUsed):
			writeError(w, http.StatusConflict, err.Error())
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, "failed to undo")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
