package routes

import(
	"Leakops-backend/internal/handlers"

	"github.com/gofiber/fiber/v2"
)


func RegisterAuthRoutes(router fiber.Router, h *handlers.AuthHandler) {
	auth := router.Group("/auth")

	auth.Post("/signup", h.Signup)
	auth.Post("/login", h.Login)
}

