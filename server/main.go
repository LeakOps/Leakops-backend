package main

import(
	"log"

	"Leakops-backend/internal/config"
	"Leakops-backend/internal/db"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Load env from config
	cfg := config.LoadConfig()

	// Connect to database + auto-migrate models
	database := db.ConnectDB(cfg.DatabaseURL)
}