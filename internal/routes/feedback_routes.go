package routes

import(
	"Leakops-backend/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

func RegisterFeedbackRoutes(router fiber.Router, h *handlers.FeedbackHandler) {
	router.Post("/feedback", h.SubmitFeedback)
}
