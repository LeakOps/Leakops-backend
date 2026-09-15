package handlers

import(
	"errors"
	"log"
	"strings"

	"Leakops-backend/internal/gateway"
	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"
 
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)


type WebhookHandler struct {
	DB 				*gorm.DB
	EncryptionKey	string
}


func NewWebhookHandler(db *gorm.DB, encryptionKey string) *WebhookHandler {
	return &WebhookHandler{DB: db, EncryptionKey: encryptionKey}
}

// Route must be registered exactly as:
//
//	app.Post("/api/v1/webhook/:gatewayType/:accountID", h.HandleWebhook)
//
// Fiber param names are case-sensitive — a mismatch here silently yields empty
// strings rather than an error.
func(h* WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	gatewayTypeStr := strings.ToLower(c.Params("gatwayType")) // "stripe" | "dodo"
	accountIDStr := c.Params("accountID")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid gateway account id",
		})
	}

	var gatewayAccount models.GatewayAccount
	if err := h.DB.Where("id = ? AND is_active = ?", accountID, true).First(&gatewayAccount).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "gateway account not found",
		})
	}
	// FIX: the gateway type came straight from the URL and was never checked
	// against the stored account. Hitting /webhook/stripe/<dodo-account-id>
	// would hand a Dodo secret to the Stripe verifier. Verification would fail,
	// so it was not exploitable — but it produced confusing 400s and was simply
	// wrong. The path and the account must agree.
	if string(gatewayAccount.GatewayType) != gatewayTypeStr {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "gateway account not found",
		})
	}

	// Defence in depth: is_active should already guarantee a secret exists, but
	// Decrypt("") is not something we want to reach if that invariant ever slips.
	if gatewayAccount.WebhookSecret == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "gateway account not configured",
		})
	}

	webhookSecret, err := utils.Decrypt(gatewayAccount.WebhookSecret, h.EncryptionKey)
	if err != nil {
		log.Printf("webhook: decrypt secret failed account=%s: %v", accountID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to process gateway credentials",
		})
	}

	gw, err := gateway.GetGateway(gatewayTypeStr)
}