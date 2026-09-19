package routes

import(
	"Leakops-backend/internal/handlers"
	"Leakops-backend/internal/middlewares"

	"github.com/gofiber/fiber/v2"
)


func RegisterProfileRoutes(router fiber.Router, h *handlers.ProfileHandler, jwtSecret string) {
	profile := router.Group("/profile", middlewares.AuthRequired(jwtSecret))
	profile.Post("/picture", h.UploadProfilePicture)
}
