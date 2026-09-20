package email

import (
	"fmt"
	"html"
	"strings"

	"github.com/resend/resend-go/v4"
)

type DunningService struct {
	client          *resend.Client
	fromEmail       string
	feedbackToEmail string
}

func NewDunningService(apiKey string, fromEmail string, feedbackToEmail string) *DunningService {
	return &DunningService{
		client:          resend.NewClient(apiKey),
		fromEmail:       fromEmail,
		feedbackToEmail: feedbackToEmail,
	}
}

func (d *DunningService) SendPaymentFailedEmail(toEmail, customerName string, amountCents int64, currency string) error {
	if toEmail == "" {
		return fmt.Errorf("recipient email is empty")
	}

	name := customerName
	if name == "" {
		name = "there"
	}

	amount := float64(amountCents) / 100

	html := fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 500px; margin: 0 auto;">
			<h2 style="color: #1a1a1a;">Payment Failed</h2>
			<p>Hi %s,</p>
			<p>We tried to charge your card for <strong>%.2f %s</strong>, but the payment didn't go through.</p>
			<p>To avoid any interruption to your service, please update your payment method as soon as possible.</p>
			<p style="color: #888; font-size: 12px; margin-top: 32px;">
				If you've already updated your payment info, please disregard this email.
			</p>
		</div>
	`, name, amount, strings.ToUpper(currency))

	params := &resend.SendEmailRequest{
		From:    d.fromEmail,
		To:      []string{toEmail},
		Subject: "Your payment failed — action needed",
		Html:    html,
	}

	_, err := d.client.Emails.Send(params)
	return err
}

func (d *DunningService) SendFeedbackNotification(name, email, message string) error {
	if d.feedbackToEmail == "" {
		return fmt.Errorf("feedback recipient email is empty")
	}

	html := fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 500px;">
			<h2>New Feedback Received</h2>
			<p><strong>From:</strong> %s (%s)</p>
			<p><strong>Message:</strong></p>
			<p>%s</p>
		</div>
	`, html.EscapeString(name), html.EscapeString(email), html.EscapeString(message))

	params := &resend.SendEmailRequest{
		From:    d.fromEmail,
		To:      []string{d.feedbackToEmail},
		Subject: "New LeakOps Feedback",
		Html:    html,
	}

	_, err := d.client.Emails.Send(params)

	return err
}
