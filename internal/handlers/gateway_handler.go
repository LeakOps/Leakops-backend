// Connect/Disconnect gateway logic
package handlers

import(
	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GatewayHandler struct {
	DB 				*gorm.DB
	EncryptionKey	string
}


func NewGatewayHandler(db *gorm.DB, encryptionKey string) *GatewayHandler {
	return &GatewayHandler{DB: db, EncryptionKey: encryptionKey}
}

type ConnectGatewayRequest struct {
	GatewayType			string		`json:"gateway_type"`  // "dodo" or "stripe"
	APIKey				string		`json:"api_key"`
	WebhookSecret 		string 		`json:"webhook_secret"`
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

func (h* GatewayHandler) ConnectGateway(c *fiber.Ctx) error {
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

	encryptedWebhookSecret := ""
	if req.WebhookSecret != "" {
		encryptedWebhookSecret, err := utils.Encrypt(req.WebhookSecret, h.EncryptionKey)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to secure webhook secret",
			})
		}
	}

	gatewayAccount := models.GatewayAccount {
		UserID: 		userID,
		GatewayType: 	models.GatewayType(req.GatewayType),
		APIKey: 		encryptedKey,
		WebhookSecret: 	encryptedWebhookSecret,
		IsActive: 		true,
	}

	if err := h.DB.Create(&gatewayAccount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to connect gateway",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "gateway connected successfully",
		"gateway": fiber.Map{
			"id": 				gatewayAccount.ID,
			"gateway_type":		gatewayAccount.GatewayType,
			"is_active":		gatewayAccount.IsActive,
		},
	})
}