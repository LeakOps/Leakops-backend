package handlers

import(
	"encoding/json"
	"log"
	"time"

	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BillingWebhookHandler struct {
	DB 				*gorm.DB
	WebhookSecret 	string
}

func NewBillingWebhookHandler(db *gorm.DB, webhookSecret string) *BillingWebhookHandler {
	return &BillingWebhookHandler{DB: db, WebhookSecret: webhookSecret}
}

// dodoBillingEvent mirrors the shape of LeakOps's own subscription events
// from Dodo (payment/subscription lifecycle),not to be confused with the
// founder-facing payment.failed events in gateway/dodo.go.
type dodoBillingEvent struct {
	Type	string  `json:"type"`
	Data struct {
		SubscriptionID 		string `json:"subscription_id"`
		CustomerID			string `json:"customer_id"`
		Email				string `json:"email"`
		ProductID			string `json:"product_id"`
		Status 				string `json:"status"`
		NextBillingDate 	string `json:"next_billing_date"`
	} `json:"data"`
}


func (h *BillingWebhookHandler) HandleDodoBillingWebhook(c *fiber.Ctx) error {
	webhookID := c.Get("webhook-id")
	webhookTimestamp := c.Get("webhook-timestamp")
	webhookSignature := c.Get("webhook-signature")

	if err := utils.VerifyStandardWebhook(c.Body(), webhookID, webhookSignature, webhookTimestamp, h.WebhookSecret); err != nil {
		log.Printf("billing webhook: verification failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webhook verification failed",
		})
	}

	var event dodoBillingEvent
	if err := json.Unmarshal(c.Body(), &event); err != nil {
		log.Printf("billing webhook: parse failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid payload",
		})
	}

	switch event.Type {

	}
}