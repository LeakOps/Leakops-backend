package routes

import (
	"Leakops-backend/internal/handlers"
	"Leakops-backend/internal/middlewares"

	"github.com/gofiber/fiber/v2"
)

func RegisterGatewayRoutes(router fiber.Router, h *handlers.GatewayHandler, jwtSecret string) {
	gateway := router.Group("/gateway", middlewares.AuthRequired(jwtSecret))

	gateway.Post("/connect", h.ConnectGateway)
	gateway.Get("/", h.ListGateways)
	gateway.Delete("/:id", h.DisconnectGateway)
}
