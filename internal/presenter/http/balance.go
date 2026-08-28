package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

type balanceResponse struct {
	Current   loyalty.Amount `json:"current"`
	Withdrawn loyalty.Amount `json:"withdrawn"`
}

type withdrawalRequest struct {
	Order string         `json:"order"`
	Sum   loyalty.Amount `json:"sum"`
}

type withdrawalResponse struct {
	Order       string         `json:"order"`
	Sum         loyalty.Amount `json:"sum"`
	ProcessedAt string         `json:"processed_at"`
}

func (s *Server) getBalance(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	account, err := s.balance.Get(r.Context(), userID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balanceResponse{Current: account.Current, Withdrawn: account.Withdrawn})
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	var request withdrawalRequest
	if err := decoder.Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userID, _ := userIDFromContext(r.Context())
	err := s.balance.Withdraw(r.Context(), userID, request.Order, request.Sum)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, loyalty.ErrInsufficientFunds):
		w.WriteHeader(http.StatusPaymentRequired)
	case errors.Is(err, order.ErrInvalidNumber):
		w.WriteHeader(http.StatusUnprocessableEntity)
	case errors.Is(err, loyalty.ErrInvalidAmount):
		w.WriteHeader(http.StatusBadRequest)
	default:
		s.internalError(w, err)
	}
}

func (s *Server) listWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	items, err := s.balance.Withdrawals(r.Context(), userID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	response := make([]withdrawalResponse, 0, len(items))
	for _, item := range items {
		response = append(response, withdrawalResponse{Order: item.Order, Sum: item.Amount, ProcessedAt: item.ProcessedAt.Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, response)
}
