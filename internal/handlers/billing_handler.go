package handlers

import (
	"log"
	"net/url"
	"strings"

	"Leakops-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type paymentService interface {
	CreateCheckoutSession(email, productID, successURL string) (string, error)
}

type BillingHandler struct {
	DB             *gorm.DB
	PaymentSvc     paymentService
	FrontendURL    string
	ProductStarter string
	ProductGrowth  string
	ProductScale   string
}

func NewBillingHandler(db *gorm.DB, PaymentSvc paymentService, frontendURL, starter, growth, scale string) *BillingHandler {
	return &BillingHandler{
		DB: db, PaymentSvc: PaymentSvc, FrontendURL: frontendURL, ProductStarter: starter, ProductGrowth: growth, ProductScale: scale,
	}
}

func (h *BillingHandler) CreateCheckout(c *fiber.Ctx) error {
	userIDStr := c.Locals("userID").(string)
	userID, _ := uuid.Parse(userIDStr)

	var req struct {
		Plan string `json:"plan"` // "starter" | "growth" | "scale"
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	var productID string
	switch req.Plan {
	case "starter":
		productID = h.ProductStarter
	case "growth":
		productID = h.ProductGrowth
	case "scale":
		productID = h.ProductScale
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid plan",
		})
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user",
		})
	}

	checkoutURL, err := h.PaymentSvc.CreateCheckoutSession(user.Email, productID, h.FrontendURL+"/billing/success")
	if err != nil {
		log.Printf("billing: checkout session creation failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to create checkout session",
			"details": err.Error(),
		})
	}
	if strings.TrimSpace(checkoutURL) == "" {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "billing provider returned an empty checkout URL",
		})
	}
	parsedURL, err := url.Parse(checkoutURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "billing provider returned an invalid checkout URL",
		})
	}

	return c.JSON(fiber.Map{"checkout_url": checkoutURL})
}

func (h *BillingHandler) GetSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("userID").(string)
	userID, _ := uuid.Parse(userIDStr)

	var sub models.Subscription
	err := h.DB.First(&sub, "user_id = ?", userID).Error
	if err != nil {
		return c.JSON(fiber.Map{
			"plan": "free", "status": "active",
		})
	}

	return c.JSON(fiber.Map{
		"plan":               sub.Plan,
		"status":             sub.Status,
		"current_period_end": sub.CurrentPeriodEnd,
	})
}

func (h *BillingHandler) ContactSalesForEnterprise(c *fiber.Ctx) error {
	var req struct {
		CompanyName string `json:"company_name"`
		Message     string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	userIDStr := c.Locals("userID").(string)
	userID, _ := uuid.Parse(userIDStr)

	var user models.User
	h.DB.First(&user, "id = ?", userID)

	return c.JSON(fiber.Map{"message": "Our team will reach out to you shortly"})
}
