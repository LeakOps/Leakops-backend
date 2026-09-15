package gateway

import "strings"

// ParsedWebhookEvent is the common format every gateway (Stripe, Dodo, Razorpay)
// converts its webhook payload into.
type ParsedWebhookEvent struct {
	EventType          string // "payment_failed" / "payment_succeeded"
	ExternalInvoiceID  string
	ExternalCustomerID string
	CustomerEmail      string
	CustomerName       string
	AmountCents        int64
	Currency           string
	FailureReason      string
}

// PaymentGateway is implemented by every supported gateway.
type PaymentGateway interface {
	// VerifyAndParseWebhook verifies the webhook signature and converts the
	// payload into ParsedWebhookEvent.
	//
	// Contract:
	//   (event, nil) -> handled event, caller should persist it
	//   (nil,   nil) -> valid but unhandled event type, caller returns 200 OK
	//   (nil,  err)  -> verification / parsing failed, caller returns 4xx
	//
	// Returning (nil, nil) for unhandled events is important: if we returned an
	// error, Stripe/Dodo would treat the delivery as failed, retry repeatedly,
	// and eventually disable the webhook endpoint.
	//
	// `headers` is a map instead of a single signature string because Stripe
	// needs only "Stripe-Signature", while Dodo uses the Standard Webhooks
	// format which needs "webhook-id", "webhook-signature" and
	// "webhook-timestamp" together (signature + replay protection).
	VerifyAndParseWebhook(payload []byte, headers map[string]string, webhookSecret string) (*ParsedWebhookEvent, error)

	// RetryPayment retries the payment via the gateway API using the
	// tenant-specific apiKey. The key must never be stored in global state.
	RetryPayment(apiKey string, externalInvoiceID string) error
}

// headerValue does a case-insensitive lookup in the headers map.
//
// HTTP header names are case-insensitive, but Go map keys are not. Fiber's
// c.Get() is case-insensitive, so the map is built correctly today — this
// helper just makes the gateways immune to a future typo/casing change in the
// handler (e.g. "Webhook-Id" vs "webhook-id").
func headerValue(headers map[string]string, key string) string {
	if v, ok := headers[key]; ok && v != "" {
		return v
	}

	for k, v := range headers {
		if strings.EqualFold(k, key) && v != "" {
			return v
		}
	}

	return ""
}