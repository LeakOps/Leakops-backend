package routes

import(
	"Leakops-backend/internal/handlers"
	"Leakops-backend/internal/middlewares"

	"github.com/gofiber/fiber/v2"
)


func RegisterDashboardRoutes(router fiber.Router, h *handlers.DashboardHandler, jwtSecret string) {
	dashboard := router.Group("/dashboard", middlewares.AuthRequired(jwtSecret))

	dashboard.Get("/summary", h.GetSummary)
	dashboard.Get("/payments", h.GetPayments)
}
