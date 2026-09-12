package handlers

import(
	"context"
	"encoding/json"
	"io"
	"net/http"

	"Leakops-backend/internal/config"
	"Leakops-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)


type OAuthHandler struct {
	DB  			*gorm.DB
	JWTSecret		string
	GoogleConfig	*oauth2.Config
	GithubConfig	*oauth2.Config
	FrontendURL  	string
}


func NewOAuthHandler(db *gorm.DB, cfg *config.Config) *OAuthHandler {
	googleConfig := &oauth2.Config{
		ClientID: 			cfg.GoogleClientID,
		ClientSecret: 		cfg.GoogleClientSecret,
		RedirectURL: 		"http://127.0.0.1:8080/api/v1/auth/google/callback",
		Scopes: 			[]string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint: 			github.Endpoint,
	}

	githubConfig := &oauth2.Config{
		ClientID: 			cfg.GithubClientID,
		ClientSecret: 		cfg.GithubClientSecret,
		RedirectURL: 		"http://127.0.0.1:8080/api/v1/auth/github/callback",
		Scopes: 			[]string{"read:user", "user:email"},
		Endpoint: 			github.Endpoint,
	}

	return &OAuthHandler{
		DB: 			db,
		JWTSecret: 		cfg.JWTSecret,
		GoogleConfig: 	googleConfig,
		GithubConfig: 	githubConfig,
		FrontendURL: 	cfg.FrontendURL,
	}
}


// --- Google ---

func (h *OAuthHandler) GoogleLogin(c *fiber.Ctx) error {
	url := h.GoogleConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	return c.Redirect(url)
}

