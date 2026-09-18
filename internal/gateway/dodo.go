package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"Leakops-backend/internal/utils"

	"github.com/dodopayments/dodopayments-go"
	"github.com/dodopayments/dodopayments-go/option"
)


type DodoGateway struct{}


func NewDodoGateway() *DodoGateway {
	return &DodoGateway{}
}

var _ PaymentGateway = (*DodoGateway)(nil)


type dodoWebhookPayload struct {
	Type string `json:"type"`

	Data struct {
		PaymentID    string `json:"payment_id"`
		TotalAmount  int64  `json:"total_amount"`
		Currency     string `json:"currency"`
		ErrorMessage string `json:"error_message"`

		Customer *struct {
			CustomerID string `json:"customer_id"`
			Email      string `json:"email"`
			Name       string `json:"name"`
		} `json:"customer"`

		CustomerID    string `json:"customer_id"`
		CustomerEmail string `json:"customer_email"`
		CustomerName  string `json:"customer_name"`
	} `json:"data"`
}

func (p *dodoWebhookPayload) customerFields() (string, string, string) {
	if p.Data.Customer != nil {
		return p.Data.Customer.CustomerID,
			p.Data.Customer.Email,
			p.Data.Customer.Name
	}
	return p.Data.CustomerID, p.Data.CustomerEmail, p.Data.CustomerName
}


// Dodo uses the Standard Webhooks signing format.
	//
	// The signed content is:
	//
	//   webhook-id.webhook-timestamp.raw-payload
	
func (d *DodoGateway) VerifyAndParseWebhook(
	payload []byte,
	headers map[string]string,
	webhookSecret string,
) (*ParsedWebhookEvent, error) {

	webhookID := headerValue(headers, "webhook-id")
	webhookSignature := headerValue(headers, "webhook-signature")
	webhookTimestamp := headerValue(headers, "webhook-timestamp")

	// Signature verification now lives in utils.VerifyStandardWebhook,
	// shared with the LeakOps billing webhook handler — both use the same
	// Standard Webhooks signing scheme.
	if err := utils.VerifyStandardWebhook(payload, webhookID, webhookTimestamp, webhookSignature, webhookSecret); err != nil {
		return nil, err
	}

	var event dodoWebhookPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	if event.Type != "payment.failed" {
		return nil, nil
	}

	customerID, customerEmail, customerName := event.customerFields()

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

func (d *DodoGateway) RetryPayment(apiKey string, externalInvoiceID string) error {
	return errors.New("dodo payment retry is not implemented")
}

func (d *DodoGateway) RegisterWebhook(apiKey string, webhookURL string) (string, error) {
	if apiKey == "" {
		return "", errors.New("missing dodo api key")
	}

	clientOptions := []option.RequestOption{
		option.WithBearerToken(apiKey),
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DODO_ENVIRONMENT")), "test") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("DODO_ENVIRONMENT")), "test_mode") {
		clientOptions = append(clientOptions, option.WithEnvironmentTestMode())
	}

	client := dodopayments.NewClient(clientOptions...)

	webhookDetails, err := client.Webhooks.New(context.Background(), dodopayments.WebhookNewParams{
		URL: dodopayments.F(webhookURL),
		FilterTypes: dodopayments.F([]dodopayments.WebhookEventType{
			dodopayments.WebhookEventTypePaymentFailed,
		}),
	})
	if err != nil {
		return "", err
	}

	if webhookDetails.ID == "" {
		return "", errors.New("dodo did not return a webhook id")
	}

	secretResponse, err := client.Webhooks.GetSecret(context.Background(), webhookDetails.ID)
	if err != nil {
		return "", err
	}

	if secretResponse.Secret == "" {
		return "", errors.New("dodo did not return a webhook secret")
	}

	return secretResponse.Secret, nil
}
