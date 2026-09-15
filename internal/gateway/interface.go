package gateway

// ParsedWebhookEvent is a common format in which every gateway (Stripe, Dodo, Razorpay) convert their webhook data.

type ParsedWebhookEvent struct {
	EventType				string		// "payment_failed" ya "payment_succeeded"
	ExternalInvoiceID		string
	ExternalCustomerID		string
	CustomerEmail			string
	CustomerName			string
	AmountCents				int64
	Currency				string
	FailureReason			string
}


type PaymentGateway interface {
// VerifyAndParseWebhook verifies the webhook signature and converts the payload
// into a common format.
//
// FIX: If the event type is not handled by our system (for example,
// "customer.subscription.updated"), this function now returns (nil, nil)
// instead of an error. The caller (webhook_handler.go) treats this as a
// 200 OK response. Previously, returning an error caused Stripe to consider
// the webhook delivery failed and retry it repeatedly. After too many failed
// deliveries, Stripe could automatically disable the webhook endpoint.
// Ignoring unhandled events is the correct behavior.
//
// UPDATE: `signatureHeader string` has been replaced with
// `headers map[string]string`.
//
// Reason: Stripe only requires a single header ("Stripe-Signature"), while
// Dodo uses the Standard Webhooks format, which requires THREE headers:
// "webhook-id", "webhook-signature", and "webhook-timestamp". These headers
// are used together to provide proper signature verification and protection
// against replay attacks.
//
// Using a generic headers map allows each gateway to extract the headers it
// requires while keeping the interface uniform across all payment gateways.

	VerifyAndParseWebhook(payload []byte, headers map[string]string, webhookSecret string) (*ParsedWebhookEvent, error)

	// Retries the payment by making an API call to the payment gateway.
	RetryPayment(apiKey string, externalInvoiceID string) error
}

