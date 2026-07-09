package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/db"
	"github.com/franciskershaw/packing-list-go/internal/auth"
	"github.com/franciskershaw/packing-list-go/internal/handler"
	"github.com/franciskershaw/packing-list-go/internal/middleware"
	"github.com/franciskershaw/packing-list-go/internal/repository"
	"github.com/gin-gonic/gin"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
		os.Exit(1)
	}

	// Initialise the DB
	err = db.InitDB()
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
	authHandler := handler.NewAuthHandler(userRepo, oauthManager, cfg)

	// Initialize Gin server
	server := gin.Default()

	// Health check (public)
	server.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to the Packing List API",
		})
	})

	// Auth routes (public)
	server.GET("/auth/google/login", authHandler.LoginWithGoogle)
	server.GET("/auth/google/callback", authHandler.GoogleCallback)
	server.POST("/auth/refresh", authHandler.RefreshToken)
	server.POST("/auth/logout", authHandler.Logout)

	// Authenticated routes
	categoryHandler := handler.NewCategoryHandler(repository.NewCategoryRepository(db.DB))
	itemRepo := repository.NewItemRepository(db.DB)
	itemHandler := handler.NewItemHandler(itemRepo)
	templateHandler := handler.NewTemplateHandler(repository.NewTemplateRepository(db.DB), itemRepo)
	authed := server.Group("/")
	authed.Use(middleware.AuthMiddleware())
	{
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
		authed.POST("/templates/:id/items/bulk", templateHandler.BulkAddItems)
	}

	// TODO: Register packing list routes

	if err := server.Run(":" + cfg.Port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
