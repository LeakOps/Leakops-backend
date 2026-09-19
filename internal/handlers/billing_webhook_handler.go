package handlers

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BillingWebhookHandler struct {
	DB             *gorm.DB
	WebhookSecret  string
	ProductStarter string
	ProductGrowth  string
	ProductScale   string
}

func NewBillingWebhookHandler(db *gorm.DB, webhookSecret, productStarter, productGrowth, productScale string) *BillingWebhookHandler {
	return &BillingWebhookHandler{
		DB:             db,
		WebhookSecret:  webhookSecret,
		ProductStarter: productStarter,
		ProductGrowth:  productGrowth,
		ProductScale:   productScale,
	}
}

// dodoBillingEvent mirrors the shape of LeakOps's own subscription events
// from Dodo (payment/subscription lifecycle),not to be confused with the
// founder-facing payment.failed events in gateway/dodo.go.
type dodoBillingEvent struct {
	Type string `json:"type"`
	Data struct {
		SubscriptionID  string `json:"subscription_id"`
		CustomerID      string `json:"customer_id"`
		Email           string `json:"email"`
		ProductID       string `json:"product_id"`
		Status          string `json:"status"`
		NextBillingDate string `json:"next_billing_date"`
	} `json:"data"`
}

func (h *BillingWebhookHandler) HandleDodoBillingWebhook(c *fiber.Ctx) error {
	webhookID := c.Get("webhook-id")
	webhookTimestamp := c.Get("webhook-timestamp")
	webhookSignature := c.Get("webhook-signature")

	if err := utils.VerifyStandardWebhook(c.Body(), webhookID, webhookTimestamp, webhookSignature, h.WebhookSecret); err != nil {
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
	case "subscription.active", "subscription.renewed", "payment.succeeded":
		h.upsertSubscription(event, models.SubStatusActive)
	case "subscription.cancelled", "subscription.expired":
		h.upsertSubscription(event, models.SubStatusCancelled)
	case "payment.failed", "subscription.failed", "subscription.past_due", "subscription.on_hold":
		h.upsertSubscription(event, models.SubStatusPastDue)
	case "subscription.paused":
		h.upsertSubscription(event, models.SubStatusCancelled)
	default:
		log.Printf("billing webhook: unhandled event type %s", event.Type)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *BillingWebhookHandler) upsertSubscription(event dodoBillingEvent, status models.SubscriptionStatus) {
	if event.Data.Email == "" {
		log.Printf("billing webhook: event %s missing customer email, cannot map to user", event.Type)
		return
	}

	var user models.User
	if err := h.DB.Where("email = ?", event.Data.Email).First(&user).Error; err != nil {
		log.Printf("billing webhook: no user found for email %s", event.Data.Email)
		return
	}

	plan := h.planFromProductID(event.Data.ProductID)

	var sub models.Subscription
	result := h.DB.Where("user_id = ?", user.ID).First(&sub)

	if result.Error != nil {
		sub = models.Subscription{
			ID:                 uuid.New(),
			UserID:             user.ID,
			Plan:               plan,
			Status:             status,
			DodoSubscriptionID: event.Data.SubscriptionID,
			DodoCustomerID:     event.Data.CustomerID,
		}

		if err := h.DB.Create(&sub).Error; err != nil {
			log.Printf("billing webhook: failed to create subscription: %v", err)
		}
		return
	}

	updates := map[string]interface{}{
		"status":               status,
		"plan":                 plan,
		"dodo_subscription_id": event.Data.SubscriptionID,
		"dodo_customer_id":     event.Data.CustomerID,
	}

	if event.Data.NextBillingDate != "" {
		if t, err := time.Parse(time.RFC3339, event.Data.NextBillingDate); err == nil {
			updates["current_period_end"] = t
		}
	}

	h.DB.Model(&sub).Updates(updates)
}

// planFromProductID maps a Dodo product ID back to our PlanTier enum.
func (h *BillingWebhookHandler) planFromProductID(productID string) models.PlanTier {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return models.PlanFree
	}

	switch productID {
	case h.ProductStarter:
		return models.PlanStarter
	case h.ProductGrowth:
		return models.PlanGrowth
	case h.ProductScale:
		return models.PlanScale
	default:
		return models.PlanFree
	}
}
