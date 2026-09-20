package main

import (
	"log"

	"Leakops-backend/internal/config"
	"Leakops-backend/internal/db"
	"Leakops-backend/internal/email"
	"Leakops-backend/internal/middlewares"
	"Leakops-backend/internal/retry"
	"Leakops-backend/internal/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	// Load env from config
	cfg := config.LoadConfig()

	// Connect to database + auto-migrate models
	database := db.ConnectDB(cfg.DatabaseURL)

	// Create a fiber app
	app := fiber.New()

	// Adding middlewares
	app.Use(cors.New(middlewares.SetupCORS(cfg.FrontendURL)))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Leakops backend is running")
	})

	// Register all Routes
	routes.SetupRoutes(app, database, cfg)

	// Starting retry engine in background (Day 1/3/7 retries + dunning emails)
	dunningSvc := email.NewDunningService(cfg.ResendAPIKey, cfg.ResendFromEmail, cfg.FeedbackToEmail)
	retryEngine := retry.NewEngine(database, cfg.EncryptionKey, dunningSvc)
	go retryEngine.Start()

	log.Println("Leakops backend listening on port :" + cfg.PORT)
	log.Fatal(app.Listen(":" + cfg.PORT))
}
