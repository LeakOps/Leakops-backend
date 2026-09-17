package retry

import (
	"log"
	"time"

	"Leakops-backend/internal/email"
	"Leakops-backend/internal/gateway"
	"Leakops-backend/internal/models"
	"Leakops-backend/internal/utils"

	"gorm.io/gorm"
)

type Engine struct {
	DB            *gorm.DB
	EncryptionKey string
	DunningSvc    *email.DunningService
}

func NewEngine(db *gorm.DB, encryptionKey string, dunningSvc *email.DunningService) *Engine {
	return &Engine{DB: db, EncryptionKey: encryptionKey, DunningSvc: dunningSvc}
}

func (e *Engine) Start() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	log.Println("Retry engine started")
	for range ticker.C {
		e.processDuePayments()
	}
}

func (e *Engine) processDuePayments() {
	var due []models.FailedPayment
	err := e.DB.Where(
		"status IN ? AND next_try_at IS NOT NULL AND next_try_at <= ?", []string{string(models.StatusPending), string(models.StatusRetrying)}, time.Now()).Find(&due).Error

	if err != nil {
		log.Printf("retry engine: fetch failed: %v", err)
		return
	}

	for _, p := range due {
		e.retryOne(p)
	}
}

func (e *Engine) retryOne(payment models.FailedPayment) {
	var ga models.GatewayAccount
	if err := e.DB.First(&ga, "id = ?", payment.GatewayAccountID).Error; err != nil {
		log.Printf("retry enginer: gateway account missing for %s: %v", payment.ID, err)
		return
	}

	apiKey, err := utils.Decrypt(ga.APIKey, e.EncryptionKey)
	if err != nil {
		log.Printf("retry engine: decrypt failed for %s: %v", payment.ID, err)
		return
	}

	gw, err := gateway.GetGateway(string(ga.GatewayType))
	if err != nil {
		log.Printf("retry engine: unsupported gateway for %s: %v", payment.ID, err)
		return
	}

	retryErr := gw.RetryPayment(apiKey, payment.ExternalInvoiceID)
	attempt := payment.RetryCount + 1
	success := retryErr == nil

	logEntry := models.RetryLog{
		FailedPaymentID: payment.ID,
		AttemptNumber:   attempt,
		Success:         success,
		AttemptedAt:     time.Now(),
	}

	if retryErr != nil {
		logEntry.ErrorMessage = retryErr.Error()
	}
	e.DB.Create(&logEntry)

	var customer models.Customer
	e.DB.First(&customer, "id = ?", payment.CustomerID)

	if success {
		e.DB.Model(&payment).Updates(map[string]interface{}{
			"status": string(models.StatusRecovered), "retry_count": attempt, "next_try_at": nil,
		})
		log.Printf("retry engine: %s RECOVERED", payment.ID)
		return
	}

	delay, more := nextRetryDelay(attempt)
	if !more {
		e.DB.Model(&payment).Updates(map[string]interface{}{
			"status": string(models.StatusChurned), "retry_count": attempt, "next_try_at": nil,
		})
		log.Printf("retry engine: %s CHURNED", payment.ID)
		return
	}

	next := time.Now().Add(delay)
	e.DB.Model(&payment).Updates(map[string]interface{}{
		"status": string(models.StatusRetrying), "retry_count": attempt, "next_try_at": next,
	})
	log.Printf("retry engine: %s next retry at %v", payment.ID, next)

	if e.DunningSvc != nil && customer.Email != "" {
		if err := e.DunningSvc.SendPaymentFailedEmail(customer.Email, customer.Name, payment.AmountCents, payment.Currency); err != nil {
			log.Printf("retry engine: dunning email failed for %s: %v", payment.ID, err)
		}
	}
}
