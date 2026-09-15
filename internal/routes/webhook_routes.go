package routes

import(
	"Leakops-backend/internal/handlers"
 
	"github.com/gofiber/fiber/v2"
)


// RegisterWebhookRoutes wires the Stripe/Dodo webhook endpoint.
//
// IMPORTANT: no middlewares.AuthRequired here, and it must stay that way.
// Stripe and Dodo cannot send a bearer token — authentication for this route
// IS the signature check inside VerifyAndParseWebhook, combined with the
// unguessable account UUID in the path. Putting this behind AuthRequired
// means every delivery gets a 401, the gateway retries a few times, then
// disables the webhook endpoint on their side.
func RegisterWebhookRoutes(router fiber.Router, h *handlers.WebhookHandler) {
	webhook := router.Group("/webhook")

	webhook.Post("/:gatewayType/:accountID", h.HandleWebhook)
}
