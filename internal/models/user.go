package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthProvider string

const (
	ProviderEmail  AuthProvider = "email"
	ProviderGoogle AuthProvider = "google"
	ProviderGithub AuthProvider = "github"
)

type User struct {
	ID                uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	Name              string       `gorm:"not null" json:"name"`
	Email             string       `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash      string       `json:"-"`
	Provider          AuthProvider `gorm:"not null;default:'email'" json:"provider"`
	ProviderID        string       `json:"-"` // Google/GitHub Unique user ID
	ProfilePictureURL string       `json:"profile_picture_url"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`

	GatewayAccounts []GatewayAccount `gorm:"foreignKey:UserID" json:"-"`
}

type OAuthIdentity struct {
	ID         uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID    `gorm:"type:uuid;not null" json:"user_id"`
	Provider   AuthProvider `gorm:"not null;uniqueIndex:idx_oauth_identity" json:"provider"`
	ProviderID string       `gorm:"not null;uniqueIndex:idx_oauth_identity" json:"-"`
	User       User         `gorm:"foreignKey:UserID" json:"-"`
}

func (i *OAuthIdentity) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
