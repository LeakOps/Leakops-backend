package routes

import(
	"Leakops-backend/internal/handlers"

	"github.com/gofiber/fiber/v2"
)


func RegisterOAuthRoutes(router fiber.Router, h *handlers.OAuthHandler) {
	router.Get("/auth/google", h.GoogleLogin)
	router.Get("/auth/google/callback", h.GoogleCallback)

	router.Get("/auth/github", h.GithubLogin)
	router.Get("/auth/github/callback", h.GithubCallback)
}
