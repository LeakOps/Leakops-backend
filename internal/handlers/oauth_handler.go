package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"Leakops-backend/internal/config"
	"Leakops-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"gorm.io/gorm"
)

const oauthCallTimeout = 10 * time.Second

type OAuthHandler struct {
	DB           *gorm.DB
	JWTSecret    string
	GoogleConfig *oauth2.Config
	GithubConfig *oauth2.Config
	FrontendURL  string
}


func NewOAuthHandler(db *gorm.DB, cfg *config.Config) *OAuthHandler {
	googleConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  "http://127.0.0.1:8080/api/v1/auth/google/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     github.Endpoint,
	}

	githubConfig := &oauth2.Config{
		ClientID:     cfg.GithubClientID,
		ClientSecret: cfg.GithubClientSecret,
		RedirectURL:  "http://127.0.0.1:8080/api/v1/auth/github/callback",
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
	}

	return &OAuthHandler{
		DB:           db,
		JWTSecret:    cfg.JWTSecret,
		GoogleConfig: googleConfig,
		GithubConfig: githubConfig,
		FrontendURL:  cfg.FrontendURL,
	}
}

// generateState is a random, unguessable state string which is used for CSRF protection
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// --- Google ---

func (h *OAuthHandler) GoogleLogin(c *fiber.Ctx) error {
	state, err := generateState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to start oauth flow"})
	}
 

	// Storing cookie in httpOnly to verify callback
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state_google",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})
 
	url := h.GoogleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return c.Redirect(url)
}



type googleUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	ID    string `json:"id"`
}

func (h *OAuthHandler) GoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "missing code",
		})
	}

	// CSRF protection: verifying state
	state := c.Query("state")
	cookieState := c.Cookies("oauth_state_google")

	if state == "" || cookieState == "" || state != cookieState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid oauth state",
		})
	}
	c.ClearCookie("oauth_state_google")

	ctx, cancel := context.WithTimeout(context.Background(), oauthCallTimeout)
	defer cancel()

	token, err := h.GoogleConfig.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to exchange token",
		})
	}

	client := h.GoogleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch user info",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "google userinfo request failed",
		})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to read user info",
		})
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to parse user info",
		})
	}

	return h.findOrCreateOAuthUser(c, info.Email, info.Name, info.ID, models.ProviderGoogle)
}


// --- GitHub ---

func (h *OAuthHandler) GithubLogin(c *fiber.Ctx) error {
	state, err := generateState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to start oauth flow",
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:  		"oauth_state_github",
		Value: 		state,
		Expires: 	time.Now().Add(10 *time.Minute),
		HTTPOnly: 	true,
		Secure: 	true,
		SameSite: 	"Lax",
	})

	url := h.GithubConfig.AuthCodeURL(state)
	return c.Redirect(url)
}

type githubUserInfo struct {
	Login 		string		`json:"login"`
	Name		string		`json:"name"`
	ID 			int			`json:"id"`
	Email		string		`json:"email"`
}

func (h *OAuthHandler) GithubCallback(c *fiber.Ctx) error {
	code := c.Query("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing code",
		})
	}

	state := c.Query("state")
	cookieState := c.Cookies("oauth_state_github")

	if state == "" || cookieState == "" || state != cookieState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid oauth state",
		})
	}
	c.ClearCookie("oauth_state_github")

	ctx, cancel := context.WithTimeout(context.Background(), oauthCallTimeout)
	defer cancel()

	token, err := h.GithubConfig.Exchange(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to exchange token",
		})
	}

	client := h.GithubConfig.Client(ctx, token)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to build request",
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch user info",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "github userinfo request failed",
		})
	}
}