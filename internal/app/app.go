package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	accrualadapter "github.com/GLEZH/gofermart-graduate-project/internal/adapter/accrual"
	"github.com/GLEZH/gofermart-graduate-project/internal/adapter/postgres"
	"github.com/GLEZH/gofermart-graduate-project/internal/adapter/security"
	"github.com/GLEZH/gofermart-graduate-project/internal/config"
	httpapi "github.com/GLEZH/gofermart-graduate-project/internal/presenter/http"
	"github.com/GLEZH/gofermart-graduate-project/internal/presenter/worker"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
	usecaseauth "github.com/GLEZH/gofermart-graduate-project/internal/usecase/auth"
	"github.com/GLEZH/gofermart-graduate-project/internal/usecase/balance"
	"github.com/GLEZH/gofermart-graduate-project/internal/usecase/orders"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	server *http.Server
	worker *worker.Accrual
	db     *postgres.Database
	log    *zap.Logger
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg == nil || strings.TrimSpace(cfg.RunAddress) == "" {
		return nil, errors.New("run address is empty")
	}
	if strings.TrimSpace(cfg.AccrualSystemAddress) == "" {
		return nil, errors.New("accrual system address is empty")
	}
	if strings.TrimSpace(cfg.AuthSecret) == "" {
		return nil, errors.New("authentication secret is empty")
	}
	if cfg.AccrualPollInterval <= 0 {
		cfg.AccrualPollInterval = time.Second
	}
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	database, err := postgres.NewDatabase(ctx, cfg.DatabaseURI)
	if err != nil {
		_ = logger.Sync()
		return nil, err
	}
	if err = database.Migrate(); err != nil {
		_ = database.Close()
		_ = logger.Sync()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	store := postgres.NewStore(database.SQLDB())
	tokens := security.NewTokenManager(cfg.AuthSecret)
	authService := usecaseauth.NewService(store, security.NewPasswordHasher(0), tokens)
	orderService := orders.NewService(store)
	balanceService := balance.NewService(store)
	calculator := accrualadapter.NewClient(cfg.AccrualSystemAddress, nil)
	processor := accrualusecase.NewProcessor(calculator, store)
	sugar := logger.Sugar()
	httpServer := httpapi.NewServer(authService, orderService, balanceService, tokens, sugar)

	return &App{
		server: &http.Server{
			Addr: cfg.RunAddress, Handler: httpServer.Router(), ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
		},
		worker: worker.NewAccrual(processor, cfg.AccrualPollInterval, sugar),
		db:     database,
		log:    logger,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	group.Go(func() error {
		return a.worker.Run(groupCtx)
	})
	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	})
	return group.Wait()
}

func (a *App) Close() error {
	dbErr := a.db.Close()
	logErr := a.log.Sync()
	if dbErr != nil {
		return dbErr
	}
	return logErr
}
