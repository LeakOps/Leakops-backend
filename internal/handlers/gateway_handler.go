// Connect/Disconnect gateway logic
package handlers

import (
	"errors"
	"strings"
	"time"

	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"

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

// minAPIKeyLength guards the APIKeyLastFour slice below. Without it, a 3-char
// key would be stored in full as "last four" — i.e. the whole secret sitting in
// plaintext in a column meant to be safe to display.
const minAPIKeyLength = 8

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
			"error": "unauthorized",
		})
	}

	var req ConnectGatewayRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	req.GatewayType = strings.ToLower(strings.TrimSpace(req.GatewayType))
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.WebhookSecret = strings.TrimSpace(req.WebhookSecret)

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

	if len(req.APIKey) < minAPIKeyLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_key looks invalid",
		})
	}

	// Duplicate check: same user, same gateway type already connected?
	//
	// FIX: previously only `err == nil` was handled. Any other error (DB down,
	// connection reset) was silently swallowed and the code carried on to create
	// a second row. Now non-"not found" errors return 500.
	var existing models.GatewayAccount
	err = h.DB.Where("user_id = ? AND gateway_type = ?", userID, req.GatewayType).First(&existing).Error

	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "gateway already connected, disconnect it first",
		})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to connect gateway",
		})
	}

	// TODO: validate the key against the gateway API here before storing it.

	encryptedKey, err := utils.Encrypt(req.APIKey, h.EncryptionKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to secure api key",
		})
	}

	lastFour := req.APIKey[len(req.APIKey)-4:]

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
		// FIX: this used to be hardcoded `true`. webhook_handler.go looks up
		// accounts with `is_active = true`, so an account with an EMPTY webhook
		// secret would match and then blow up inside Decrypt("") / HMAC key
		// derivation. IsActive is now true only when a secret actually exists;
		// otherwise SetWebhookSecret activates it later.
		IsActive:    encryptedWebhookSecret != "",
		ConnectedAt: time.Now(),
	}

	if err := h.DB.Create(&gatewayAccount).Error; err != nil {
		// The composite unique index on (user_id, gateway_type) is the real
		// guarantee against duplicates — the SELECT above can lose a race
		// between two concurrent requests. Report that race as a 409, not a 500.
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "gateway already connected, disconnect it first",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "gateway connected successfully",
		"gateway": fiber.Map{
			"id":           gatewayAccount.ID,
			"gateway_type": gatewayAccount.GatewayType,
			"is_active":    gatewayAccount.IsActive,
			// The founder registers this URL in their Stripe/Dodo dashboard,
			// gets a signing secret back, and posts it to SetWebhookSecret.
			"webhook_url": "/api/v1/webhook/" + string(gatewayAccount.GatewayType) + "/" + gatewayAccount.ID.String(),
		},
	})
}

// SetWebhookSecret — Step B of the connect flow. The founder registers the
// webhook_url returned by ConnectGateway in their Stripe/Dodo dashboard,
// receives a signing secret, and submits it here. This activates the account.
//
// FIX: this method did not exist before, so there was no way to set the secret
// after account creation — and supplying it during ConnectGateway is impossible
// in the correct order, since the secret only exists after the webhook URL has
// been registered.
func (h *GatewayHandler) SetWebhookSecret(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// FIX: parse the path param instead of passing a raw string into the query.
	// A malformed uuid otherwise reaches Postgres and produces a 500 from an
	// invalid-input-syntax error instead of a clean 404.
	gatewayID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid gateway id",
		})
	}

	var req struct {
		WebhookSecret string `json:"webhook_secret"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	req.WebhookSecret = strings.TrimSpace(req.WebhookSecret)
	if req.WebhookSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webhook_secret is required",
		})
	}

	var gatewayAccount models.GatewayAccount
	// user_id is part of the WHERE clause on purpose. Without it any
	// authenticated user could guess another founder's gatewayID and overwrite
	// their webhook secret — a classic IDOR.
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

	// Targeted UPDATE instead of Save(&struct). Save writes every column, which
	// would also rewrite APIKey/ConnectedAt and could clobber a concurrent
	// change. Only the two fields that actually change are touched here.
	if err := h.DB.Model(&gatewayAccount).Updates(map[string]interface{}{
		"webhook_secret": encryptedSecret,
		"is_active":      true,
	}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save webhook secret",
		})
	}

	return c.JSON(fiber.Map{
		"message":   "webhook secret saved, gateway is now active",
		"id":        gatewayAccount.ID,
		"is_active": true,
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

	response := make([]fiber.Map, 0, len(gateways))
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

	gatewayID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid gateway id",
		})
	}

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
