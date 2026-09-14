package gateway

import (
	"context"
	"encoding/json"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

type StripeGateway struct{}

func NewStripeGateway() *StripeGateway {
	return &StripeGateway{}
}

func (s *StripeGateway) VerifyAndParseWebhook(payload []byte, signatureHeader string, webhookSecret string) (*ParsedWebhookEvent, error) {
	event, err := webhook.ConstructEvent(payload, signatureHeader, webhookSecret)
	if err != nil {
		return nil, err
	}

	switch event.Type {
	case "invoice.payment_failed":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return nil, err
		}

		// FIX: Previously invoice.Customer.ID was being accessed OUTSIDE this
		// nil-check (in the return statement below), which would cause a nil
		// pointer panic if Customer was nil. Now customerID is also being
		// safely extracted inside this same nil-check.
		customerEmail := ""
		customerName := ""
		customerID := ""
		if invoice.Customer != nil {
			customerEmail = invoice.Customer.Email
			customerName = invoice.Customer.Name
			customerID = invoice.Customer.ID
		}

		failureReason := ""
		if invoice.LastFinalizationError != nil {
			failureReason = invoice.LastFinalizationError.Msg
		}

		return &ParsedWebhookEvent{
			EventType:          "payment_failed",
			ExternalInvoiceID:  invoice.ID,
			ExternalCustomerID: customerID,
			CustomerEmail:      customerEmail,
			CustomerName:       customerName,
			AmountCents:        invoice.AmountDue,
			Currency:           string(invoice.Currency),
			FailureReason:      failureReason,
		}, nil

	default:
		// FIX: Previously, this returned `errors.New("unhandled event type...")`,
		// which caused the webhook_handler to return a 400 response. Stripe would
		// then keep retrying the event, potentially causing the endpoint to be
		// disabled. Unhandled events are now silently ignored (nil, nil), so the
		// handler treats them as a 200 OK response.
		return nil, nil
	}
}

func (s *StripeGateway) RetryPayment(apiKey string, externalInvoiceID string) error {
	
	// FIX: Previously, `stripe.Key = apiKey` was being set here. This is a
	// package-level GLOBAL variable. In a multi-tenant system, if two founders'
	// retry operations ran concurrently, one request could potentially use
	// another founder's API key due to a race condition, creating a risk of
	// charging or retrying against the wrong Stripe account.
	// Now, we create a per-request isolated client so the `apiKey` remains
	// scoped only to this call, with no shared or global state.

	// Client.API() this is deprecated
	sc := stripe.NewClient(apiKey)
	// Retry the invoice payment using the tenant-specific Stripe client.
	_, err := sc.V1Invoices.Pay(
		context.Background(), externalInvoiceID, nil,
	)

	return err
}