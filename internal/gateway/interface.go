package gateway

import "strings"


type ParsedWebhookEvent struct {
	EventType          string
	ExternalInvoiceID  string
	ExternalCustomerID string
	CustomerEmail      string
	CustomerName       string
	AmountCents        int64
	Currency           string
	FailureReason      string
}


type PaymentGateway interface {
	VerifyAndParseWebhook(payload []byte, headers map[string]string, webhookSecret string) (*ParsedWebhookEvent, error)
	RetryPayment(apiKey string, externalInvoiceID string) error

	// RegisterWebhook creates a webhook endpoint on the gateway's side for the
	// given URL, and returns the signing secret the gateway generates.
	// This removes the need for founders to manually create webhooks and
	// paste secrets back — we do it for them at connect time.
	RegisterWebhook(apiKey string, webhookURL string) (string, error)
}

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