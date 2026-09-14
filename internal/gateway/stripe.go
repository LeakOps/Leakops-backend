package gateway

import(
	"encoding/json"
	"errors"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/client"
	"github.com/stripe/stripe-go/v86/webhook"
)


type StripeGateway struct {}

func NewStripeGateway() *StripeGateway {
	return &StripeGateway{}
}

func