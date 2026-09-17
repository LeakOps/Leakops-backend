package routes

import (
	"Leakops-backend/internal/config"
	"Leakops-backend/internal/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)


func SetupRoutes(app *fiber.App, database *gorm.DB, cfg *config.Config) {
	api := app.Group("/api/v1")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(database, cfg.JWTSecret)
	oauthHandler := handlers.NewOAuthHandler(database, cfg)
	gatewayHandler := handlers.NewGatewayHandler(database, cfg.EncryptionKey, cfg.BaseURL)
	webhookHandler := handlers.NewWebhookHandler(database, cfg.EncryptionKey)
	dashboardHandler := handlers.NewDashboardHandler(database)

	
	// Register route groups
	RegisterAuthRoutes(api, authHandler)
	RegisterOAuthRoutes(api, oauthHandler)
	RegisterGatewayRoutes(api, gatewayHandler, cfg.JWTSecret)
	RegisterWebhookRoutes(api, webhookHandler)
	RegisterDashboardRoutes(api, dashboardHandler, cfg.JWTSecret)
}

