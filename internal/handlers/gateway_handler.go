// Connect/Disconnect gateway logic
package handlers

import (
	"errors"
	"strings"
	"time"

	"Leakops-backend/internal/gateway"
	"Leakops-backend/internal/models"
	"Leakops-backend/internal/services"
	"Leakops-backend/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GatewayHandler struct {
	DB            *gorm.DB
	EncryptionKey string
	BaseURL       string // e.g. https://api.leakops.com or ngrok URL in dev
}

func NewGatewayHandler(db *gorm.DB, encryptionKey, baseURL string) *GatewayHandler {
	return &GatewayHandler{DB: db, EncryptionKey: encryptionKey, BaseURL: baseURL}
}

// ConnectGatewayRequest — the founder now only sends api_key. webhook_secret
// is no longer requested manually; the backend registers with the gateway
// API itself and fetches the secret (see ConnectGateway Step 3).
type ConnectGatewayRequest struct {
	GatewayType string `json:"gateway_type"` // "dodo" or "stripe"
	APIKey      string `json:"api_key"`
}

// minAPIKeyLength guards the APIKeyLastFour slice below. Without it, a 3-char
// key would be stored in full as "last four" — i.e. the whole secret sitting in
// plaintext in a column meant to be safe to display.
const minAPIKeyLength = 8

var errGatewayLimitReached = errors.New("gateway limit reached")

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

// ConnectGateway — the founder only provides an api_key. The backend:
//  1. Creates the record (is_active=false, webhook_secret empty)
//  2. Builds the webhook URL from that record's ID
//  3. Calls the Stripe/Dodo API to register the webhook and gets a secret back
//  4. Encrypts the secret and updates the same record, is_active=true
//
// On every failure point the record is rolled back (deleted) — no half-baked
// row is left behind in the DB.
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

	plan, err := services.UserPlan(h.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load subscription",
		})
	}
	limits := services.LimitsForPlan(plan)
	if limits.MaxGateways > 0 {
		var gatewayCount int64
		if err := h.DB.Model(&models.GatewayAccount{}).Where("user_id = ?", userID).Count(&gatewayCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to check gateway limit",
			})
		}
		if gatewayCount >= int64(limits.MaxGateways) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":        "gateway_limit_reached",
				"message":      "your current plan does not allow another payment gateway",
				"current_plan": plan,
				"limit":        limits.MaxGateways,
			})
		}
	}

	// Duplicate check: same user, same gateway type already connected?
	// Non-"not found" errors (DB down, connection reset) return 500 instead
	// of silently falling through and creating a second row.
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

	encryptedKey, err := utils.Encrypt(req.APIKey, h.EncryptionKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to secure api key",
		})
	}

	lastFour := req.APIKey[len(req.APIKey)-4:]

	// Step 1: create the row. IsActive=false, webhook_secret empty — don't
	// treat the account as active until the webhook is actually registered.
	// Otherwise webhook_handler.go will pick it up with an empty secret and
	// blow up inside Decrypt("") / HMAC key derivation.
	gatewayAccount := models.GatewayAccount{
		UserID:         userID,
		GatewayType:    models.GatewayType(req.GatewayType),
		APIKey:         encryptedKey,
		APIKeyLastFour: lastFour,
		IsActive:       false,
		ConnectedAt:    time.Now(),
	}

	createErr := h.DB.Transaction(func(tx *gorm.DB) error {
		// Serialize gateway creation per user so two concurrent requests cannot
		// both pass the plan's gateway-count check.
		var lockedUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedUser, "id = ?", userID).Error; err != nil {
			return err
		}

		if limits.MaxGateways > 0 {
			var gatewayCount int64
			if err := tx.Model(&models.GatewayAccount{}).Where("user_id = ?", userID).Count(&gatewayCount).Error; err != nil {
				return err
			}
			if gatewayCount >= int64(limits.MaxGateways) {
				return errGatewayLimitReached
			}
		}

		return tx.Create(&gatewayAccount).Error
	})
	if errors.Is(createErr, errGatewayLimitReached) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":        "gateway_limit_reached",
			"message":      "your current plan does not allow another payment gateway",
			"current_plan": plan,
			"limit":        limits.MaxGateways,
		})
	}
	if createErr != nil {
		// The composite unique index on (user_id, gateway_type) is the real
		// guarantee against duplicates. Report that race as a 409, not a 500.
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "gateway already connected, disconnect it first",
		})
	}

	// Step 2: build the webhook URL — it depends on this record's ID, so it
	// can only be built after creation.
	webhookURL := h.BaseURL + "/api/v1/webhook/" + req.GatewayType + "/" + gatewayAccount.ID.String()

	// Step 3: register the webhook with the gateway API. We use the plaintext
	// req.APIKey here (the encrypted version is already stored in the DB).
	gw, err := gateway.GetGateway(req.GatewayType)
	if err != nil {
		h.DB.Delete(&gatewayAccount) // rollback — half-baked row mat chhodo
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported gateway",
		})
	}

	secret, err := gw.RegisterWebhook(req.APIKey, webhookURL)
	if err != nil {
		h.DB.Delete(&gatewayAccount) // rollback — galat API key ya gateway-side issue
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "failed to register webhook, check your API key: " + err.Error(),
		})
	}

	// Step 4: encrypt the secret and do a targeted UPDATE — not Save(), since
	// that rewrites every column (APIKey/ConnectedAt too), which could
	// clobber a concurrent change. Only touch the fields that actually change.
	encryptedSecret, err := utils.Encrypt(secret, h.EncryptionKey)
	if err != nil {
		h.DB.Delete(&gatewayAccount) // rollback
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to secure webhook secret",
		})
	}

	if err := h.DB.Model(&gatewayAccount).Updates(map[string]interface{}{
		"webhook_secret": encryptedSecret,
		"is_active":      true,
	}).Error; err != nil {
		h.DB.Delete(&gatewayAccount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to activate gateway",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "gateway connected and webhook registered automatically",
		"gateway": fiber.Map{
			"id":           gatewayAccount.ID,
			"gateway_type": gatewayAccount.GatewayType,
			"is_active":    true,
		},
	})
}

// SetWebhookSecret — manual fallback. In the normal flow, ConnectGateway
// itself registers the webhook and sets the secret (Step 3-4 above). This
// endpoint is useful when auto-registration fails for some reason (gateway
// API down, rate-limited, permissions issue) and the founder needs to
// manually grab the secret from the Stripe/Dodo dashboard and set it here.
func (h *GatewayHandler) SetWebhookSecret(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Parse the path param instead of passing a raw string into the query.
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
