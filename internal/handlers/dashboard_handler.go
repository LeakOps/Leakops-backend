package handlers

import (
	"Leakops-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	DB *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{DB: db}
}

// GetSummary returns high-level numbers for the founder's dashboard:
// revenue at risk, recovered amount, and recovery rate.
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Locals("userID").(string))

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user",
		})
	}

	var atRiskCents int64
	h.DB.Model(&models.FailedPayment{}).
		Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
		Where("gateway_accounts.user_id = ? AND failed_payments.status IN ?", userID, []string{"pending", "retrying"}).
		Select("COALESCE(SUM(failed_payments.amount_cents), 0)").
		Scan(&atRiskCents)

	var recoveredCents int64
	h.DB.Model(&models.FailedPayment{}).
		Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
		Where("gateway_accounts.user_id = ? AND failed_payments.status = ?", userID, "recovered").
		Select("COALESCE(SUM(failed_payments.amount_cents), 0)").
		Scan(&recoveredCents)

	var totalCount, recoveredCount int64
	h.DB.Model(&models.FailedPayment{}).
		Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
		Where("gateway_accounts.user_id = ?", userID).
		Count(&totalCount)

	h.DB.Model(&models.FailedPayment{}).
		Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
		Where("gateway_accounts.user_id = ? AND failed_payments.status = ?", userID, "recovered").
		Count(&recoveredCount)

		recoveryRate := 0.0
		if totalCount > 0 {
			recoveryRate = float64(recoveredCount) / float64(totalCount) * 100
		}

		return c.JSON(fiber.Map{
			"revenue_at_risk_cents": atRiskCents,
			"recovered_cents":       recoveredCents,
			"recovery_rate":		 recoveryRate,
			"total_failed_payments": totalCount,
		})
}


// GetPayments returns the list of all failed payments for the founder,
// most recent first, with customer info attached.
func (h *DashboardHandler) GetPayments(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Locals("userID").(string))

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user",
		})
	}

	var payments []models.FailedPayment
	h.DB.Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
	Where("gateway_accounts.user_id = ?", userID).
	Preload("Customer").
	Order("failed_payments.created_at DESC").
	Find(&payments)

	response := make([]fiber.Map, 0)
	for _, p := range payments {
		response = append(response, fiber.Map{
			"id":				p.ID,
			"customer_name":	p.Customer.Name,
			"customer_email": p.Customer.Email,
			"amount_cents":   p.AmountCents,
			"currency":		  p.Currency,
			"status":		  p.Status,
			"retry_count":	  p.RetryCount,
			"next_try_at":    p.NextTryAt,
			"created_at":     p.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{"payments": response})
}
