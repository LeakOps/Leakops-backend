package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)


type GatewayType string

const (
	GatewayStripe	GatewayType = "stripe"
	GatewayDodo		GatewayType = "dodo"
)


type GatewayAccount struct {
	ID  			uuid.UUID		 `gorm:"type:uuid;primaryKey" json:"id"`
	UserID			uuid.UUID		 `gorm:"type:uuid;not null;index" json:"user_id"`
	GatewayType		GatewayType		 `gorm:"not null" json:"gateway_type"`
	APIKey			string			 `gorm:"not null" json:"-"`   // encrypted before save
	WebhookSecret	string			 `json:"-"`
	IsActive		bool			 `gorm:"default:true" json:"is_active"`
	ConnectedAt		time.Time		 `json:"connected_at"`
	UpdatedAt 		time.Time 		 `json:"updated_at"`

	User  User	`gorm:"foreignKey:UserID" json:"-"`
}

func (g *GatewayAccount) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}

	return
}

