package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GatewayType is an enum so `models.GatewayDodo` / `models.GatewayStripe`
// compile correctly and typos ("strip", "Dodo") are caught by the type checker.
type GatewayType string

const (
	GatewayStripe GatewayType = "stripe"
	GatewayDodo   GatewayType = "dodo"
)


type GatewayAccount struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	// FIX: composite unique index added. The duplicate check in ConnectGateway
	// is a SELECT-then-INSERT, which two concurrent requests can both pass.
	// The DB constraint is the only real guarantee of one account per
	// (user, gateway type).
	UserID      uuid.UUID   `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_gateway_type" json:"user_id"`
	GatewayType GatewayType `gorm:"not null;uniqueIndex:idx_user_gateway_type" json:"gateway_type"`

	APIKey        string `gorm:"not null" json:"-"` // encrypted
	WebhookSecret string `json:"-"`                 // encrypted; empty until SetWebhookSecret

	// Last 4 chars of the raw API key so the UI can show which key is connected
	// without ever re-displaying the secret.
	APIKeyLastFour string `json:"api_key_last_four"`

	IsActive    bool      `gorm:"default:false;index" json:"is_active"`
	ConnectedAt time.Time `json:"connected_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (g *GatewayAccount) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return
}

