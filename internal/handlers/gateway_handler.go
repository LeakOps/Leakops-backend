// Connect/Disconnect gateway logic
package handlers

import (
	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GatewayHandler struct {
	DB            *gorm.DB
	EncryptionKey string
}

func NewGatewayHandler(db *gorm.DB, encryptionKey string) *GatewayHandler {
	return &GatewayHandler{DB: db, EncryptionKey: encryptionKey}
}

type ConnectGatewayRequest struct {
	GatewayType   string `json:"gateway_type"` // "dodo" or "stripe"
	APIKey        string `json:"api_key"`
	WebhookSecret string `json:"webhook_secret"`
}

// getUserID safely pulls the authenticated user's ID out of locals.
// Panic-proof: if middleware locals is not set, it returns 401.
func getUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userIDStr, ok := c.Locals("userID").(string)
	if !ok || userIDStr == "" {
		return uuid.UUID{}, fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.UUID{}, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
	}

	return userID, nil
}

func (h *GatewayHandler) ConnectGateway(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user",
		})
	}

	var req ConnectGatewayRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.GatewayType != string(models.GatewayDodo) && req.GatewayType != string(models.GatewayStripe) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "gateway_type must be 'dodo' or 'stripe'",
		})
	}

	if req.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_key is required",
		})
	}

	// Duplicate check: same user, same gateway type already connected?
	var existing models.GatewayAccount
	err = h.DB.Where("user_id = ? AND gateway_type = ?", userID, req.GatewayType).First(&existing).Error
	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "gateway already connected, disconnect it first",
		})
	}

	// TODO: A validation call should be made to the gateway API here
	// to verify that the key is valid. Skip it for now and add it later.

	encryptedKey, err := utils.Encrypt(req.APIKey, h.EncryptionKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to secure api key",
		})
	}

	lastFour := req.APIKey
	if len(lastFour) > 4 {
		lastFour = lastFour[len(lastFour)-4:]
	}

	encryptedWebhookSecret := ""
	if req.WebhookSecret != "" {
		encryptedWebhookSecret, err = utils.Encrypt(req.WebhookSecret, h.EncryptionKey)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to secure webhook secret",
			})
		}
	}

	gatewayAccount := models.GatewayAccount{
		UserID:         userID,
		GatewayType:    models.GatewayType(req.GatewayType),
		APIKey:         encryptedKey,
		APIKeyLastFour: lastFour,
		WebhookSecret:  encryptedWebhookSecret,
		// FIX: Previously, this was always set to `true`, regardless of whether
		// WebhookSecret was empty or not. This caused webhook_handler.go (which
		// searches for accounts using the `is_active = true` filter) to match
		// this account. However, since WebhookSecret was empty, signature
		// verification could fail or crash when calling Decrypt("") or deriving
		// the HMAC key. Now, IsActive will only be set to true if a webhook
		// secret is provided in this request. Otherwise, the account will be
		// activated later through the SetWebhookSecret endpoint (see below).
		IsActive:    req.WebhookSecret != "",
		ConnectedAt: time.Now(),
	}

	if err := h.DB.Create(&gatewayAccount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to connect gateway",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "gateway connected successfully",
		"gateway": fiber.Map{
			"id":           gatewayAccount.ID,
			"gateway_type": gatewayAccount.GatewayType,
			"is_active":    gatewayAccount.IsActive,
			// The founder will register this URL in their Stripe/Dodo dashboard.
			// They will then receive a webhook secret, which must be sent back
			// through the SetWebhookSecret endpoint.
			"webhook_url": "/api/v1/webhook/" + string(gatewayAccount.GatewayType) + "/" + gatewayAccount.ID.String(),
		},
	})
}

// SetWebhookSecret — Step B of the connect flow. The founder registers the
// webhook_url returned by ConnectGateway in their Stripe/Dodo dashboard,
// receives a signing secret, and submits it here. This activates the
// account to receive webhook events.
//
// FIX: This method was previously missing, so there was no way to set the
// webhook secret after the account was created. The only option was to
// provide the secret during ConnectGateway, which is not possible in the
// correct order because the secret is only generated after the webhook URL
// has already been registered.

func (h *GatewayHandler) SetWebhookSecret(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	gatewayID := c.Params("id")

	var req struct {
		WebhookSecret string `json:"webhook_secret"`
	}
	if err := c.BodyParser(&req); err != nil || req.WebhookSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webhook_secret is required",
		})
	}

	var gatewayAccount models.GatewayAccount
	// user_id is also being validated. Otherwise, any authenticated user could
	// guess another founder's gatewayID and overwrite their webhook secret,
	// resulting in an IDOR vulnerability.
	if err := h.DB.Where("id = ? AND user_id = ?", gatewayID, userID).First(&gatewayAccount).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "gateway not found",
		})
	}

	encryptedSecret, err := utils.Encrypt(req.WebhookSecret, h.EncryptionKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to secure webhook secret",
		})
	}

	gatewayAccount.WebhookSecret = encryptedSecret
	gatewayAccount.IsActive = true

	if err := h.DB.Save(&gatewayAccount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save webhook secret",
		})
	}

	return c.JSON(fiber.Map{
		"message":   "webhook secret saved, gateway is now active",
		"id":        gatewayAccount.ID,
		"is_active": gatewayAccount.IsActive,
	})
}

func (h *GatewayHandler) ListGateways(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var gateways []models.GatewayAccount
	if err := h.DB.Where("user_id = ?", userID).Find(&gateways).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch gateways",
		})
	}

	response := make([]fiber.Map, 0)
	for _, g := range gateways {
		response = append(response, fiber.Map{
			"id":           g.ID,
			"gateway_type": g.GatewayType,
			"is_active":    g.IsActive,
			"connected_at": g.ConnectedAt,
			"last_four":    g.APIKeyLastFour,
		})
	}

	return c.JSON(fiber.Map{"gateways": response})
}

func (h *GatewayHandler) DisconnectGateway(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	gatewayID := c.Params("id")

	result := h.DB.Where("id = ? AND user_id = ?", gatewayID, userID).Delete(&models.GatewayAccount{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to disconnect gateway",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "gateway not found",
		})
	}

	return c.JSON(fiber.Map{"message": "gateway disconnected"})
}
