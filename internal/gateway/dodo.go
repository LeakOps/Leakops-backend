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

type DodoGateway struct{}

func NewDodoGateway() *DodoGateway {
	return &DodoGateway{}
}

// Compile-time guarantee that DodoGateway satisfies PaymentGateway.
var _ PaymentGateway = (*DodoGateway)(nil)

// dodoWebhookPayload accepts BOTH shapes of the customer block:
//
//	nested : data.customer.{customer_id,email,name}   <- current Dodo format
//	flat   : data.{customer_id,customer_email,customer_name}
//
// FIX: only the flat shape was handled before. If Dodo sends the nested shape,
// json.Unmarshal does NOT error — the three fields just stay empty strings, and
// every failed payment gets recorded against a single customer row with an
// empty external_customer_id. Silent data corruption. Handling both shapes and
// rejecting an empty id (below) makes that impossible.
type dodoWebhookPayload struct {
	Type string `json:"type"`

	Data struct {
		PaymentID    string `json:"payment_id"`
		TotalAmount  int64  `json:"total_amount"`
		Currency     string `json:"currency"`
		ErrorMessage string `json:"error_message"`

		// Nested shape (preferred).
		Customer *struct {
			CustomerID string `json:"customer_id"`
			Email      string `json:"email"`
			Name       string `json:"name"`
		} `json:"customer"`

		// Flat shape (fallback).
		CustomerID    string `json:"customer_id"`
		CustomerEmail string `json:"customer_email"`
		CustomerName  string `json:"customer_name"`
	} `json:"data"`
}

// customerFields returns (id, email, name) from whichever shape was sent.
func (p *dodoWebhookPayload) customerFields() (string, string, string) {
	if p.Data.Customer != nil {
		return p.Data.Customer.CustomerID,
			p.Data.Customer.Email,
			p.Data.Customer.Name
	}
 
	return p.Data.CustomerID, p.Data.CustomerEmail, p.Data.CustomerName
}

// Allowed clock skew between Dodo's timestamp and our clock.
const dodoTimestampTolerance = 5 * time.Minute

func (d *DodoGateway) VerifyAndParseWebhook(
	payload []byte,
	headers map[string]string,
	webhookSecret string,
) (*ParsedWebhookEvent, error) {

	// Dodo uses the Standard Webhooks format: three headers together.
	webhookID := headerValue(headers, "webhook-id")
	webhookSignature := headerValue(headers, "webhook-signature")
	webhookTimestamp := headerValue(headers, "webhook-timestamp")

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

	if eventTime.Before(now.Add(-dodoTimestampTolerance)) ||
		eventTime.After(now.Add(dodoTimestampTolerance)) {
		return nil, errors.New("webhook timestamp outside tolerance window")
	}

	// Signed content is: webhook-id.webhook-timestamp.raw-payload
	// Binding the id and timestamp into the signature is what stops a valid
	// payload from being replayed freely.
	signedPayload := webhookID + "." + webhookTimestamp + "." + string(payload)

	// Dodo signing secrets are base64. The "whsec_" prefix is not part of the
	// decoded secret.
	secret := strings.TrimPrefix(webhookSecret, "whsec_")
 
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, errors.New("invalid webhook secret")
	}

	mac := hmac.New(sha256.New, secretBytes)
	if _, err := mac.Write([]byte(signedPayload)); err != nil {
		return nil, err
	}
	expectedSignature := mac.Sum(nil)
 
	// The webhook-signature header may carry multiple signatures during a
	// secret rotation. Accept if any v1 signature matches.
	if !verifyDodoSignature(webhookSignature, expectedSignature) {
		return nil, errors.New("invalid webhook signature")
	}
 
	var event dodoWebhookPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
 
	// FIX: this used to be `if event.Type != "payment.failed" { }` — an EMPTY
	// block. It compiles with no warning, so EVERY Dodo event (payment.succeeded,
	// subscription.active, dispute.opened...) fell through and was returned as
	// EventType "payment_failed". Successful payments would land in the
	// FailedPayment table and trigger dunning emails to paying customers.
	//
	// Now unhandled events return (nil, nil) -> handler replies 200 OK, Dodo
	// does not retry them.
	if event.Type != "payment.failed" {
		return nil, nil
	}
 
	customerID, customerEmail, customerName := event.customerFields()
 
	// Guard rails: without these two ids the row would be unusable and would
	// collide with every other malformed event via FirstOrCreate.
	if customerID == "" {
		return nil, errors.New("dodo webhook missing customer id")
	}
	if event.Data.PaymentID == "" {
		return nil, errors.New("dodo webhook missing payment id")
	}
 
	return &ParsedWebhookEvent{
		EventType:          "payment_failed",
		ExternalInvoiceID:  event.Data.PaymentID,
		ExternalCustomerID: customerID,
		CustomerEmail:      customerEmail,
		CustomerName:       customerName,
		AmountCents:        event.Data.TotalAmount,
		Currency:           event.Data.Currency,
		FailureReason:      event.Data.ErrorMessage,
	}, nil
}
 
// verifyDodoSignature compares the expected MAC against every v1 signature in
// the header.
//
// Header format (space separated): "v1,<base64sig> v1,<base64sig>"
//
// FIX: comparison now happens on decoded BYTES rather than base64 strings.
// hmac.Equal on strings of different length short-circuits, and base64 padding
// differences could cause a false mismatch. Byte comparison is both correct and
// constant-time for equal-length inputs.
func verifyDodoSignature(signatureHeader string, expectedSignature []byte) bool {
	matched := false
 
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
 
		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			continue
		}
 
		// No early return: keep looping so total work does not depend on which
		// signature matched.
		if hmac.Equal(sigBytes, expectedSignature) {
			matched = true
		}
	}
 
	return matched
}
 
func (d *DodoGateway) RetryPayment(apiKey string, externalInvoiceID string) error {
	// TODO: implement Dodo's payment-retry/recovery API here.
	//
	// IMPORTANT: do NOT return nil until the real API call exists. Returning nil
	// would make LeakOps believe the payment was retried when no request was
	// ever made.
	//
	// When implementing: build the Dodo client INSIDE this method using the
	// tenant-specific apiKey. Never put the key in package-level/global state
	// (that is the exact bug that was fixed in stripe.go).
	return errors.New("dodo payment retry is not implemented")
}
