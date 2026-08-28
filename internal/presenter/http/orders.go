package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

type orderResponse struct {
	Number     string          `json:"number"`
	Status     order.Status    `json:"status"`
	Accrual    *loyalty.Amount `json:"accrual,omitempty"`
	UploadedAt string          `json:"uploaded_at"`
}

func (s *Server) submitOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	value := strings.TrimSpace(string(body))
	if value == "" || !digitsOnly(value) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = s.orders.Submit(r.Context(), userID, value)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, order.ErrOwnedByUser):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, order.ErrOwnedByAnotherUser):
		w.WriteHeader(http.StatusConflict)
	case errors.Is(err, order.ErrInvalidNumber):
		w.WriteHeader(http.StatusUnprocessableEntity)
	default:
		s.internalError(w, err)
	}
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	items, err := s.orders.List(r.Context(), userID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	response := make([]orderResponse, 0, len(items))
	for _, item := range items {
		response = append(response, orderResponse{
			Number: item.Number.String(), Status: item.Status, Accrual: item.Accrual,
			UploadedAt: item.UploadedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func digitsOnly(value string) bool {
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
