package config

import(
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL				string
	PORT					string
	JWTSecret				string
	FrontendURL				string

	DodoAPIKey				string
	DodoWebhookSecret 		string
	DodoMode				string

	StripeSecretKey			string
	StripeWebhookSecret 	string

	ResendAPIKey			string
}


func LoadConfig() *Config {
	err := godotenv.Load()

	if err != nil {
		log.Println("No .env file found, reading from system env")
	}

	dodoMode := os.Getenv("DODO_MODE")

	if dodoMode == "" {
		dodoMode = "test"
	}

	frontendURL := os.Getenv("FRONTEND_URL")

	if frontendURL == "" {
		frontendURL = "http://localhost:3000"  // local dev
	}

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		PORT: port,
		JWTSecret: os.Getenv("JWT_SECRET"),
		FrontendURL: frontendURL,

		// DODO
		DodoAPIKey: 	   os.Getenv("DODO_API_KEY"),
		DodoWebhookSecret: os.Getenv("DODO_WEBHOOK_SECRET"),
		DodoMode: 		   dodoMode,

		// Stripe
		StripeSecretKey: os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),

		//Resend Email
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set in .env")
	}

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is not set in .env")
	}

	return cfg
}

