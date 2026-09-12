package routes

import(
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

	
	// Register route groups
	RegisterAuthRoutes(api, authHandler)
	RegisterOAuthRoutes(api, oauthHandler)
}

