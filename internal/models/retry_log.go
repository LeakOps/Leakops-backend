package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)


type RetryLog struct {
	ID 				  	uuid.UUID		`gorm:"type:uuid;primaryKey" json:"id"`
	FailedPaymentID	  	uuid.UUID		`gorm:"type:uuid;not null;index" json:"failed_payment_id"`
	AttemptNumber	  	int				`gorm:"not null" json:"attempt_number"`
	Success				bool			`json:"success"`
	ErrorMessage		string			`json:"error_message"`
	AttemptedAt			time.Time		`json:"attempted_at"`

	FailedPayment  FailedPayment	`gorm:"foreignKey:FailedPaymentID" json:"-"`
}


func(r *RetryLog) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}

	return
}

