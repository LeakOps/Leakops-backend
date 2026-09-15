package gateway

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

type StripeGateway struct{}

func NewStripeGateway() *StripeGateway {
	return &StripeGateway{}
}

// Compile-time guarantee that StripeGateway satisfies PaymentGateway.
// If the interface ever changes again, this line fails to build instead of
// blowing up later at the gateway.GetGateway() call site.
var _ PaymentGateway = (*StripeGateway)(nil)

// FIX: signature was still `signatureHeader string` while the interface had
// moved to `headers map[string]string`, so *StripeGateway no longer satisfied
// PaymentGateway and the package did not compile.
func (s *StripeGateway) VerifyAndParseWebhook(
	payload []byte,
	headers map[string]string,
	webhookSecret string,
) (*ParsedWebhookEvent, error) {

	signatureHeader := headerValue(headers, "Stripe-Signature")
	if signatureHeader == "" {
		return nil, errors.New("missing Stripe-Signature header")
	}

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

		// FIX: invoice.Customer.ID was previously read outside this nil-check,
		// which panics when Customer is nil. All three fields are now read
		// inside the same guard.
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

		// Without a customer id we cannot attribute the payment to anyone.
		// Fail loudly instead of silently creating an empty-id customer row.
		if customerID == "" {
			return nil, errors.New("stripe invoice has no customer id")
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
		// Valid signature, event type we don't care about -> (nil, nil) so the
		// handler answers 200 OK and Stripe stops retrying.
		return nil, nil
	}
}

func (s *StripeGateway) RetryPayment(apiKey string, externalInvoiceID string) error {
	if apiKey == "" {
		return errors.New("missing stripe api key")
	}
	if externalInvoiceID == "" {
		return errors.New("missing stripe invoice id")
	}

	// FIX: `stripe.Key = apiKey` was a package-level GLOBAL. In a multi-tenant
	// system two concurrent retries could cross keys and charge against the
	// wrong Stripe account. A per-request client keeps the key scoped to this
	// call only.
	sc := stripe.NewClient(apiKey)

	_, err := sc.V1Invoices.Pay(context.Background(), externalInvoiceID, nil)

	return err
}