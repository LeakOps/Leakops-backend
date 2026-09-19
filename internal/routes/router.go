package routes

import (
	"Leakops-backend/internal/config"
	"Leakops-backend/internal/handlers"
	"Leakops-backend/internal/services"

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

	paymentSvc := services.NewPaymentService(cfg.LeakopsDodoAPIKey, cfg.LeakopsDodoTestMode)
	billingHandler := handlers.NewBillingHandler(database, paymentSvc, cfg.FrontendURL, cfg.DodoProductStarter, cfg.DodoProductGrowth, cfg.DodoProductScale)
	BillingWebhookHandler := handlers.NewBillingWebhookHandler(database, cfg.LeakopsDodoWebhookSecret)

	storageSvc := services.NewStorageService(
		cfg.SupabaseS3Endpoint, cfg.SupabaseS3Region,
		cfg.SupabaseS3AccessKeyID, cfg.SupabaseS3SecretKey,
		cfg.SupabaseBucketName, cfg.SupabasePublicURL,
	)
	profileHandler := handlers.NewProfileHandler(database, storageSvc)

	// Register route groups
	RegisterAuthRoutes(api, authHandler)
	RegisterOAuthRoutes(api, oauthHandler)
	RegisterGatewayRoutes(api, gatewayHandler, cfg.JWTSecret)
	RegisterWebhookRoutes(api, webhookHandler)
	RegisterDashboardRoutes(api, dashboardHandler, cfg.JWTSecret)
	RegisterBillingRoutes(api, billingHandler, BillingWebhookHandler, cfg.JWTSecret)
	RegisterProfileRoutes(api, profileHandler, cfg.JWTSecret)
}

