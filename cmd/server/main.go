package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unwise-backend/config"
	"unwise-backend/database"
	"unwise-backend/handlers"
	authmiddleware "unwise-backend/middleware"
	"unwise-backend/repository"
	"unwise-backend/services"
	"unwise-backend/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	if os.Getenv("APP_ENV") == "development" {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()
	undo := zap.ReplaceGlobals(logger)
	defer undo()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	expenseRepo := repository.NewExpenseRepository(db)
	friendRepo := repository.NewFriendRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	settlementService := services.NewSettlementService(expenseRepo, groupRepo)
	groupService := services.NewGroupService(groupRepo, userRepo, expenseRepo, settlementService, db)
	expenseService := services.NewExpenseService(expenseRepo, groupRepo, db)
	userService := services.NewUserService(userRepo, expenseRepo, cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)
	dashboardService := services.NewDashboardService(userRepo, groupRepo, expenseRepo, userService)
	friendService := services.NewFriendService(friendRepo, userRepo, groupRepo, expenseRepo, settlementService)
	commentService := services.NewCommentService(commentRepo, expenseRepo, groupRepo)

	explanationService, err := services.NewExplanationService(cfg.GeminiAPIKey, expenseRepo, groupRepo, userRepo)
	if err != nil {
		logger.Fatal("Failed to create explanation service", zap.Error(err))
	}

	receiptService, err := services.NewReceiptService(cfg.GeminiAPIKey)
	if err != nil {
		logger.Fatal("Failed to create receipt service", zap.Error(err))
	}

	storageService := storage.NewSupabaseStorage(cfg.SupabaseStorageURL, cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)

	authMiddleware := authmiddleware.NewAuthMiddleware(cfg.SupabaseJWTSecret, cfg.SupabaseURL)

	h := handlers.NewHandlers(
		groupService,
		expenseService,
		settlementService,
		receiptService,
		dashboardService,
		userService,
		explanationService,
		friendService,
		commentService,
		storageService,
		cfg.SupabaseStorageBucket,
		cfg.SupabaseGroupPhotosBucket,
		cfg.SupabaseUserAvatarsBucket,
	)

	importService := services.NewImportService(groupRepo, userRepo, expenseRepo, db)
	importHandlers := handlers.NewImportHandlers(importService)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(authmiddleware.ZapLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(authmiddleware.SecurityHeaders)
	r.Use(authmiddleware.MaxBodySize(cfg.MaxBodySize))
	if cfg.Env == "production" {
		r.Use(authmiddleware.StrictTransportSecurity)
	}

	corsOptions := cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	r.Use(cors.Handler(corsOptions))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(httprate.LimitByIP(services.GeneralRateLimit, 1*time.Minute))
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(services.AIRateLimit, 1*time.Minute))
			r.Post("/scan-receipt", h.ScanReceipt)
			r.Post("/expenses/explain", h.ExplainTransaction)
		})

		h.RegisterRoutes(r)
		importHandlers.RegisterRoutes(r)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("Server starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
