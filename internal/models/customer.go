package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)


type Customer struct {
	ID 					uuid.UUID		`gorm:"type:uuid;primaryKey" json:"id"`
	UserID				uuid.UUID		`gorm:"type:uuid;not null;index" json:"user_id"`
	GatewayAccountID	uuid.UUID		`gorm:"type:uuid;not null;uniqueIndex:idx_gateway_customer" json:"gateway_account_id"`
	ExternalCustomerID	string			`gorm:"not null;uniqueIndex:idx_gateway_customer" json:"external_customer_id"`
	Email				string			`json:"email"`
	Name				string			`json:"name"`
	CreatedAt			time.Time		`json:"created_at"`
	UpdatedAt			time.Time		`json:"updated_at"`

	GatewayAccount      GatewayAccount  `gorm:"foreignKey:GatewayAccountID" json:"-"`
}


func (c *Customer) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}

	return
}