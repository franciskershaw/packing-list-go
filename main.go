package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/franciskershaw/packing-list-go/config"
	"github.com/franciskershaw/packing-list-go/db"

	// "github.com/franciskershaw/packing-list-go/internal/router"
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

	// Initialize Gin server
	server := gin.Default()

	// Health check (public)
	server.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Kitted API",
		})
	})

	// Register routes
	// router.RegisterRoutes(server, cfg)

	// TODO: Register category routes
	// TODO: Register item routes
	// TODO: Register template routes
	// TODO: Register packing list routes

	server.Run(":" + cfg.Port)
}
