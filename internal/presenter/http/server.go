package httpapi

import (
	"context"
	"net/http"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	usecaseauth "github.com/GLEZH/gofermart-graduate-project/internal/usecase/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type authService interface {
	Register(ctx context.Context, credentials usecaseauth.Credentials) (string, error)
	Login(ctx context.Context, credentials usecaseauth.Credentials) (string, error)
}

type orderService interface {
	Submit(ctx context.Context, userID user.ID, value string) error
	List(ctx context.Context, userID user.ID) ([]order.Order, error)
}

type balanceService interface {
	Get(ctx context.Context, userID user.ID) (loyalty.Account, error)
	Withdraw(ctx context.Context, userID user.ID, orderValue string, amount loyalty.Amount) error
	Withdrawals(ctx context.Context, userID user.ID) ([]loyalty.Withdrawal, error)
}

type tokenVerifier interface {
	Verify(value string) (user.ID, error)
}

type Server struct {
	auth    authService
	orders  orderService
	balance balanceService
	tokens  tokenVerifier
	log     *zap.SugaredLogger
	cookie  string
}

func NewServer(auth authService, orders orderService, balance balanceService, tokens tokenVerifier, log *zap.SugaredLogger) *Server {
	return &Server{auth: auth, orders: orders, balance: balance, tokens: tokens, log: log, cookie: "gophermart_token"}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()
	router.Use(s.recoverer)
	router.Use(s.logging)
	router.Use(gzipMiddleware)

	router.Post("/api/user/register", s.register)
	router.Post("/api/user/login", s.login)
	router.Group(func(protected chi.Router) {
		protected.Use(s.authenticate)
		protected.Post("/api/user/orders", s.submitOrder)
		protected.Get("/api/user/orders", s.listOrders)
		protected.Get("/api/user/balance", s.getBalance)
		protected.Post("/api/user/balance/withdraw", s.withdraw)
		protected.Get("/api/user/withdrawals", s.listWithdrawals)
	})
	return router
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	if s.log != nil {
		s.log.Errorw("request failed", "error", err)
	}
	w.WriteHeader(http.StatusInternalServerError)
}
