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


func (h* GatewayHandler) ConnectGateway(c *fiber.Ctx) error {
	userIDStr := c.Locals("userID").(string)
	userID, err := uuid.Parse(userIDStr)
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
}