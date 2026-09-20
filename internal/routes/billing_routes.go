package routes

import (
	"Leakops-backend/internal/handlers"
	"Leakops-backend/internal/middlewares"

	"github.com/gofiber/fiber/v2"
)

func RegisterBillingRoutes(router fiber.Router, h *handlers.BillingHandler, wh *handlers.BillingWebhookHandler, jwtSecret string) {
	router.Post("/billing/webhook/dodo", wh.HandleDodoBillingWebhook)

	billing := router.Group("/billing", middlewares.AuthRequired(jwtSecret))
	billing.Post("/checkout", h.CreateCheckout)
	billing.Get("/subscription", h.GetSubscription)
	billing.Post("/contact-sales", h.ContactSalesForEnterprise)
}
