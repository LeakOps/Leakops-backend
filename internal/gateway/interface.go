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
	// VerifyAndParseWebhook verify signature and convert payload into common format
	VerifyAndParseWebhook(payload []byte, signatureHeader string, webhookSecret string) (*ParsedWebhookEvent, error)

	// Retries the payment by making an API call to the payment gateway.
	RetryPayment(apiKey string, externalInvoiceID string) error
}

