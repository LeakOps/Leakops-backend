// The important thing is that Dodo currently documents the webhook headers webhook-id, webhook-signature, and webhook-timestamp, and recommends Standard Webhooks verification rather than the old raw HMAC(payload) approach.

package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type DodoGateway struct {}

func NewDodoGateway() *DodoGateway {
	return &DodoGateway{}
}

type dodoWebhookPayload struct {
	Type  string	`json:"type"`

	Data struct {
		PaymentID		string	`json:"payment_id"`
		CustomerID		string	`json:"customer_id"`
		CustomerEmail	string	`json:"customer_email"`
		CustomerName	string	`json:"customer_name"`
		TotalAmount		int64	`json:"total_amount"`
		Currency		string	`json:"currency"`
		ErrorMessage 	string 	`json:"error_message"`
	} `json:"data"`
}

func (d *DodoGateway) VerifyAndParseWebhook(
	payload []byte,
	headers map[string]string,
	webhookSecret string,
) (*ParsedWebhookEvent, error) {

	// UPDATE: The interface now provides a headers map instead of a single
	// signatureHeader. The three Standard Webhooks headers required by Dodo
	// are extracted from this map.
	webhookID := headers["webhook-id"]
	webhookSignature := headers["webhook-signature"]
	webhookTimestamp := headers["webhook-timestamp"]

	if webhookID == "" || webhookSignature == "" || webhookTimestamp == "" {
		return nil, errors.New("missing webhook verification headers")
	}

	// Dodo uses the Standard Webhooks signing format.
	//
	// The signed content is:
	//
	//   webhook-id.webhook-timestamp.raw-payload
	//
	// This binds the signature to both the webhook ID and timestamp,
	// preventing a valid payload from being freely replayed.
 
	// Reject an invalid or stale timestamp before processing the event.
	timestamp, err := strconv.ParseInt(webhookTimestamp, 10, 64)
	if err != nil {
		return nil, errors.New("invalid webhook timestamp")
	}

	eventTime := time.Unix(timestamp, 0)
	now := time.Now()

	// Allow a small clock-skew window.
	const tolerance = 5 * time.Minute

	if eventTime.Before(now.Add(-tolerance)) || eventTime.After(now.Add(tolerance)) {
		return nil, errors.New("webhook timestamp outside tolerance window")
	}

	signedPayload := webhookID + "." + webhookTimestamp + "." + string(payload)

	// Dodo webhook secrets are represented as base64-encoded signing
	// secrets. The "whsec_" prefix is not part of the decoded secret.
	secret := strings.TrimPrefix(webhookSecret, "whsec_")

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, errors.New("Invalid webhook secret")
	}

	mac := hmac.New(sha256.New, secretBytes)

	if _, err := mac.Write([]byte(signedPayload)); err != nil {
		return nil, err
	}

	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// The webhook-signature header may contain multiple signatures.
	// Accept the request if any v1 signature matches.
	if !verifyDodoSignature(webhookSignature, expectedSignature) {
		return nil, errors.New("invalid webhook signature")
	}

	var event dodoWebhookPayload

	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}


	// LeakOps currently handles payment failures only.
	// Other valid Dodo events are acknowledged instead of returning
	// an error, so Dodo does not unnecessarily retry them.
	if event.Type != "payment.failed" {

	}

	return &ParsedWebhookEvent{
		EventType:			"payment_failed",
		ExternalInvoiceID:  event.Data.PaymentID,
		ExternalCustomerID: event.Data.CustomerID,
		CustomerEmail:      event.Data.CustomerEmail,
		CustomerName:       event.Data.CustomerName,
		AmountCents:        event.Data.TotalAmount,
		Currency:           event.Data.Currency,
		FailureReason:      event.Data.ErrorMessage,
	}, nil
}

func verifyDodoSignature(
	signatureHeader		string,
	expectedSignature	string,
) bool {
	// Standard Webhooks can send multiple signatures separated by spaces.
	
	// Example:
	//   v1,signature1 v1,signature2

	for _, signaturePart := range strings.Fields(signatureHeader) {
		parts := strings.SplitN(signaturePart, ",", 2)

		if len(parts) != 2 {
			continue
		}

		version := parts[0]
		signature := parts[1]

		if version != "v1" {
			continue
		}

		if hmac.Equal(
			[]byte(signature),
			[]byte(expectedSignature),
		) {
			return true
		}
	}

	return false
}

func (d *DodoGateway) RetryPayment (
	apiKey string,
	ExternalInvoiceID string,
) error {
	// TODO: Implement Dodo's payment-retry/recovery API here.
	//
	// IMPORTANT:
	// Do not return nil until the Dodo API call is actually implemented.
	// Returning nil here would make LeakOps believe that the payment was
	// successfully retried even though no request was made.
	//
	// When the retry API is implemented, create the Dodo client inside
	// this method using the tenant-specific apiKey. Do not store the key
	// in package-level/global state.

	return errors.New("Dodo payment retry is not implemented")
}
