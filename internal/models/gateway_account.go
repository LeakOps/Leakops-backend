package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GatewayType — UPDATE: defined as an enum type so that
// `models.GatewayDodo` / `models.GatewayStripe` compile correctly in
// gateway_handler.go, and invalid string literals ("strip", "Dodo", etc.)
// are caught at compile time through type checking.

type GatewayType string

const (
	GatewayStripe GatewayType = "stripe"
	GatewayDodo   GatewayType = "dodo"
)

// GatewayAccount represents a founder's connected Stripe/Dodo account.
// BYOK (Bring Your Own Key) model: the founder provides their own API key,
// while the webhook secret is provided later through the SetWebhookSecret
// step. Both values are stored encrypted.
//
// NOTE: The field names here must match gateway_handler.go
// (APIKey, WebhookSecret, ConnectedAt). The names do not have an
// "Encrypted" suffix, but the values stored in these fields must always
// be encrypted using utils.Encrypt.
//
// Never store plaintext values in these fields.

type GatewayAccount struct {
	ID          	uuid.UUID   	 `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      	uuid.UUID   	 `gorm:"type:uuid;not null;index" json:"user_id"`
	GatewayType 	GatewayType 	 `gorm:"not null" json:"gateway_type"`
	APIKey        	string 		     `gorm:"not null" json:"-"` // encrypted
	WebhookSecret 	string 			 `json:"-"`// encrypted; empty unless there is SetWebhookSecret inside it

	// Last few chars of the raw API key, so the UI can show which key is
	// connected without ever re-displaying the full secret.
	APIKeyLastFour 	string		     `json:"api_key_last_four"`

	IsActive    	bool 			 `gorm:"default:false" json:"is_active"`
	ConnectedAt 	time.Time		 `json:"connected_at"`
	UpdatedAt   	time.Time		 `json:"updated_at"`

	User  User	`gorm:"foreignKey:UserID" json:"-"`
}

func (g *GatewayAccount) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}

	return
}

