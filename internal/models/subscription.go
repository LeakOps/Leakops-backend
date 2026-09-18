package models

import(
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)


type PlanTier string

const (
	PlanFree 	PlanTier = "free"
	PlanStarter	PlanTier = "starter"
	PlanGrowth	PlanTier = "growth"
	planScale	PlanTier = "scale"
)

type SubscriptionStatus string

const (
	SubStatusActive		SubscriptionStatus = "active"
	SubStatusPastDue	SubscriptionStatus = "past_due"
	SubStatusCancelled	SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID 					uuid.UUID		   `gorm:"type:uuid;primaryKey" json:"id"`
	UserID				uuid.UUID		   `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"` // per founder per active subscription
	Plan 				PlanTier		   `gorm:"not null;default:'free'" json:"plan"`
	Status          	SubscriptionStatus `gorm:"not null;default:'active'" json:"status"`
	DodoSubscriptionID	string			   `json:"-"` // Subscription reference of dodo system
	DodoCustomerID		string			   `json:"-"`
	CurrentPeriodEnd	*time.Time		   `json:"CurrentPeriodEnd"`
	CreatedAt			time.Time		   `json:"created_at"`
	UpdatedAt			time.Time		   `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (s *Subscription) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}
