package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)


type PaymentStatus string

const (
	StatusPending	PaymentStatus = "pending"
	StatusRetrying	PaymentStatus = "retrying"
	StatusRecovered PaymentStatus = "recovered"
	StatusChurned	PaymentStatus = "churned"
)


type FailedPayment struct {
	ID 					uuid.UUID	`gorm:"type:uuid;primaryKey" json:"id"`
	CustomerID			uuid.UUID	`gorm:"type:uuid;not null;index" json:"customer_id"`
	GatewayAccountID	uuid.UUID	`gorm:"type:uuid;not null;uniqueIndex:idx_gateway_invoice" json:"gateway_account_id"`
	ExternalInvoiceID	string		`gorm:"not null;uniqueIndex:idx_gateway_invoice" json:"external_invoice_id"`
	AmountCents			int64		`gorm:"not null" json:"amount_cents"`
	Currency           string       `gorm:"not null" json:"currency"`
	Status				string		`gorm:"not null;default:'pending'" json:"status"`
	FailureReason		string		`json:"failure_reason"`
	RetryCount			int			`gorm:"default:0" json:"retry_count"`
	NextTryAt			*time.Time	`json:"next_try_at"`
	CreatedAt			time.Time	`json:"created_at"`
	UpdatedAt			time.Time	`json:"updated_at"`

	Customer Customer  `gorm:"foreignKey:CustomerID" json:"-"`
}

func (f *FailedPayment) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}

	return
}

