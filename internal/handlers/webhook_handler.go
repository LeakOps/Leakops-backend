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
	gatewayTypeStr := strings.ToLower(c.Params("gatewayType")) // "stripe" | "dodo"
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
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported gateway",
		})
	}

	// Stripe needs one header; Dodo needs the three Standard Webhooks headers.
	// A map keeps the interface uniform — each gateway picks what it needs.
	headers := map[string]string {
		"Stripe-Signature": c.Get("Stripe-Signature"),
		"webhook-id": c.Get("webhook-id"),
		"webhook-signature": c.Get("webhook-signature"),
		"webhook-timestamp": c.Get("webhook-timestamp"),
	}

	parsedEvent, err := gw.VerifyAndParseWebhook(c.Body(), headers, webhookSecret)
	if err != nil {
		// FIX: err.Error() used to be echoed back to the caller, leaking internal
		// verification details to anyone probing the endpoint. Log it, return a
		// generic message.
		log.Printf("webhook: verification failed account=%s gateway=%s: %v", accountID, gatewayTypeStr, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webhook verification failed",
		})
	}

	// (nil, nil) = valid but unhandled event type. Reply 200 so Stripe/Dodo
	// stop retrying and don't disable the endpoint.
	if parsedEvent == nil {
		return c.SendStatus(fiber.StatusOK)
	}

	// Customer find-or-create and FailedPayment create run in ONE transaction.
	// They used to be two independent writes: if the second failed, the DB was
	// left inconsistent. Both error returns are also checked now — previously a
	// failed Create left customer.ID as the zero uuid and the FailedPayment row
	// was written against it silently.
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		var customer models.Customer

		custResult := tx.Where(
			"gateway_account_id = ? AND external_customer_id = ?",
			accountID, parsedEvent.ExternalCustomerID,
		).First(&customer)

		// FIX: custResult.Error was never inspected. On a connection error
		// RowsAffected is also 0, so the code would happily create a duplicate
		// customer. Only ErrRecordNotFound may proceed to create.
		if custResult.Error != nil {
			if !errors.Is(custResult.Error, gorm.ErrRecordNotFound) {
				return custResult.Error
			}

			customer = models.Customer{
				UserID: 			gatewayAccount.UserID,
				GatewayAccountID: 	accountID,
				ExternalCustomerID: parsedEvent.ExternalCustomerID,
				Email:              parsedEvent.CustomerEmail,
				Name:               parsedEvent.CustomerName,
			}
			if err := tx.Create(&customer).Error; err != nil {
				return err
			}
		}

		// FirstOrCreate keyed on (gateway_account_id, external_invoice_id) makes
		// duplicate webhook deliveries idempotent.
		var payment models.FailedPayment
		return tx.Where(models.FailedPayment {
			GatewayAccountID:		accountID,
			ExternalInvoiceID: 		parsedEvent.ExternalInvoiceID,
		}).FirstOrCreate(&payment, models.FailedPayment{
			GatewayAccountID:  accountID,
			ExternalInvoiceID: parsedEvent.ExternalInvoiceID,
			CustomerID:        customer.ID,
			AmountCents:       parsedEvent.AmountCents,
			Currency:          parsedEvent.Currency,
			Status:            string(models.StatusPending),
			FailureReason:     parsedEvent.FailureReason,
		}).Error
	})

	if err != nil {
		log.Printf("webhook: persist failed account=%s invoice=%s: %v", accountID, parsedEvent.ExternalInvoiceID, err)
		// 500 is deliberate here: the signature was valid and this is our fault,
		// so we want the gateway to retry the delivery.
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to record payment event",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}