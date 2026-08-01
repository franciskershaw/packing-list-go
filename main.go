package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/handler"
	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/franciskershaw/packing-list-go/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	_ "github.com/joho/godotenv/autoload"
)

const tokenSweepInterval = time.Hour
const shutdownGracePeriod = 10 * time.Second
const maxRequestBodyBytes = 1 << 20 // 1 MB

var globalRateLimit = limiter.Rate{Period: time.Minute, Limit: 120}
var authLoginRateLimit = limiter.Rate{Period: time.Minute, Limit: 10}
var authRefreshRateLimit = limiter.Rate{Period: time.Minute, Limit: 30}

func main() {
	// Match Gin's own default writer (os.Stdout) so log output interleaves in order.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
		os.Exit(1)
	}

	// Initialise the DB
	err = db.InitDB(cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database init failed: %v\n", err)
		os.Exit(1)
	}
	defer db.CloseDB()

	// Initialise Google OAuth manager once at startup (makes a network call)
	oauthManager, err := auth.NewGoogleOAuthManager(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Google OAuth init failed: %v\n", err)
		os.Exit(1)
	}

	// Wire up dependencies
	userRepo := repository.NewPostgresUserRepository(db.DB)
	refreshTokenRepo := repository.NewPostgresRefreshTokenRepository(db.DB)
	authHandler := handler.NewAuthHandler(userRepo, oauthManager, refreshTokenRepo, cfg)

	// Initialize Gin server
	gin.SetMode(configureGinMode(cfg.Environment))
	server := gin.Default()
	if err := server.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		fmt.Fprintf(os.Stderr, "SetTrustedProxies failed: %v\n", err)
		os.Exit(1)
	}
	server.Use(middleware.ErrorLogger())
	server.Use(middleware.BodyLimit(maxRequestBodyBytes))
	server.Use(middleware.RateLimit(memory.NewStore(), globalRateLimit))

	// Health check (public)
	server.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to the Packing List API",
		})
	})

	// Auth routes (public). Login/callback/logout are rare, deliberate user
	// actions and get the tight limit; refresh fires on every page load for
	// session restore, so it needs real headroom.
	authLogin := server.Group("/auth")
	authLogin.Use(middleware.RateLimit(memory.NewStore(), authLoginRateLimit))
	{
		authLogin.GET("/google/login", authHandler.LoginWithGoogle)
		authLogin.GET("/google/callback", authHandler.GoogleCallback)
		authLogin.POST("/logout", authHandler.Logout)
	}

	authRefresh := server.Group("/auth")
	authRefresh.Use(middleware.RateLimit(memory.NewStore(), authRefreshRateLimit))
	{
		authRefresh.POST("/refresh", authHandler.RefreshToken)
	}

	// Authenticated routes
	categoryHandler := handler.NewCategoryHandler(repository.NewCategoryRepository(db.DB))
	itemRepo := repository.NewItemRepository(db.DB)
	itemHandler := handler.NewItemHandler(itemRepo)
	templateRepo := repository.NewTemplateRepository(db.DB)
	templateHandler := handler.NewTemplateHandler(templateRepo, itemRepo)
	packingListHandler := handler.NewPackingListHandler(repository.NewPackingListRepository(db.DB), templateRepo, itemRepo)
	authed := server.Group("/")
	authed.Use(middleware.AuthMiddleware(cfg.JWTSecretAccess))
	{
		authed.GET("/me", authHandler.Me)

		authed.GET("/categories", categoryHandler.List)
		authed.POST("/categories", categoryHandler.Create)
		authed.PATCH("/categories/:id", categoryHandler.Update)
		authed.DELETE("/categories/:id", categoryHandler.Delete)

		authed.GET("/items", itemHandler.List)
		authed.POST("/items", itemHandler.Create)
		authed.PATCH("/items/:id", itemHandler.Update)
		authed.DELETE("/items/:id", itemHandler.Delete)

		authed.GET("/templates", templateHandler.List)
		authed.POST("/templates", templateHandler.Create)
		authed.GET("/templates/:id", templateHandler.GetByID)
		authed.PATCH("/templates/:id", templateHandler.Update)
		authed.DELETE("/templates/:id", templateHandler.Delete)
		authed.POST("/templates/:id/items", templateHandler.AddItem)
		authed.PATCH("/templates/:id/items/:itemId", templateHandler.UpdateItem)
		authed.DELETE("/templates/:id/items/:itemId", templateHandler.RemoveItem)
		authed.PATCH("/templates/:id/items/bulk", templateHandler.BulkUpdateItems)

		authed.POST("/lists", packingListHandler.Create)
		authed.GET("/lists", packingListHandler.List)
		authed.GET("/lists/:id", packingListHandler.GetByID)
		authed.PATCH("/lists/:id", packingListHandler.Update)
		authed.DELETE("/lists/:id", packingListHandler.Delete)
		authed.POST("/lists/:id/unarchive", packingListHandler.Unarchive)
		authed.POST("/lists/:id/items", packingListHandler.AddItem)
		authed.PATCH("/lists/:id/items/:itemId", packingListHandler.UpdateItem)
		authed.DELETE("/lists/:id/items/:itemId", packingListHandler.RemoveItem)
		authed.PATCH("/lists/:id/items/bulk", packingListHandler.BulkUpdateItems)
		authed.POST("/lists/:id/pack-all", packingListHandler.PackAll)
		authed.POST("/lists/:id/unpack-all", packingListHandler.UnpackAll)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go runTokenSweeper(ctx, refreshTokenRepo, tokenSweepInterval, &wg)

	httpServer := newHTTPServer(":"+cfg.Port, server)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
	}

	wg.Wait()
}
