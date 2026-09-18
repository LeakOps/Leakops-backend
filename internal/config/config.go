package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	PORT        string
	JWTSecret   string
	FrontendURL string

	BaseURL string

	ResendAPIKey    string
	ResendFromEmail string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURL  string

	EncryptionKey 	   string

	DodoEnvironment    string

	LeakopsDodoAPIKey     string
	LeakopsDodoTestMode   bool

	DodoProductStarter 	  string
	DodoProductGrowth  	  string
	DodoProductScale      string
}


func LoadConfig() *Config {
	err := godotenv.Load()

	if err != nil {
		log.Println("No .env file found, reading from system env")
	}


	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000" // local dev
	}


	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080" // local dev fallback
	}

	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URI")
	if googleRedirectURL == "" {
		googleRedirectURL = baseURL + "/api/v1/auth/google/callback"
	}

	githubRedirectURL := os.Getenv("GITHUB_REDIRECT_URI")
	if githubRedirectURL == "" {
		githubRedirectURL = baseURL + "/api/v1/auth/github/callback"
	}

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		PORT:        port,
		JWTSecret:   os.Getenv("JWT_SECRET"),
		FrontendURL: frontendURL,

		BaseURL: baseURL,

		// Resend Email
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		ResendFromEmail: os.Getenv("RESEND_FROM_EMAIL"),

		// Google auth
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  googleRedirectURL,

		// Github auth
		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GithubRedirectURL:  githubRedirectURL,

		// Encryption
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),

		// dodo founder
    	DodoEnvironment: os.Getenv("DODO_ENVIRONMENT"),

		// Dodo billing
		LeakopsDodoAPIKey:   os.Getenv("LEAKOPS_DODO_API_KEY"),
		LeakopsDodoTestMode: strings.EqualFold(os.Getenv("LEAKOPS_DODO_ENVIRONMENT"), "test"),

		DodoProductStarter: os.Getenv("DODO_PRODUCT_STARTER"),
		DodoProductGrowth:  os.Getenv("DODO_PRODUCT_GROWTH"),
		DodoProductScale:   os.Getenv("DODO_PRODUCT_SCALE"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set in .env")
	}

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is not set in .env")
	}

	if len(cfg.EncryptionKey) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 characters")
	}

	return cfg
}
