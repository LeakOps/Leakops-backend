package main

import (
	"log"

	"Leakops-backend/internal/config"
	"Leakops-backend/internal/db"
	"Leakops-backend/internal/middlewares"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	// Load env from config
	cfg := config.LoadConfig()

	// Connect to database + auto-migrate models
	_ = db.ConnectDB(cfg.DatabaseURL)

	// Create a fiber app
	app := fiber.New()

	// Adding middlewares
	app.Use(cors.New(middlewares.SetupCORS(cfg.FrontendURL)))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Leakops backend is running")
	})

	// Register all Routes

	
	log.Println("Leakops backend listening on port :" + cfg.PORT)
	log.Fatal(app.Listen(":" + cfg.PORT))
}
