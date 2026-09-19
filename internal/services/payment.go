package services

import (
	"context"
	"errors"

	"github.com/dodopayments/dodopayments-go"
	"github.com/dodopayments/dodopayments-go/option"
)


// Payment service handles the Leakops billing system for subscription
type PaymentService struct {
	client *dodopayments.Client
}

func NewPaymentService(apiKey string, testMode bool) *PaymentService {
	opts := []option.RequestOption{option.WithBearerToken(apiKey)}
	if testMode {
		opts = append(opts, option.WithEnvironmentTestMode())
	}
	return &PaymentService{client: dodopayments.NewClient(opts...)}
}

// CreateCheckoutSession creates a Dodo payment link for a founder to
// subscribe to a given product/plan. Returns the checkout URL to redirect
// the founder.

func (p *PaymentService) CreateCheckoutSession(customerEmail string, productID string, successURL string) (string, error) {
	if productID == "" {
		return "", errors.New("product id is required")
	}
	if customerEmail == "" {
		return "", errors.New("customer email is required")
	}

	session, err := p.client.CheckoutSessions.New(context.Background(), dodopayments.CheckoutSessionNewParams{
		CheckoutSessionRequest: dodopayments.CheckoutSessionRequestParam{
			ProductCart: dodopayments.F([]dodopayments.ProductItemReqParam{
				{
					ProductID: dodopayments.F(productID),
					Quantity: dodopayments.F(int64(1)),
				},
			}),
			ReturnURL: dodopayments.F(successURL),
			Customer: dodopayments.F[dodopayments.CustomerRequestUnionParam](dodopayments.CustomerRequestParam{
				Email: dodopayments.F(customerEmail),
			}),
		},
	})
	if err != nil {
		return "", err
	}

	return session.CheckoutURL, nil
}
