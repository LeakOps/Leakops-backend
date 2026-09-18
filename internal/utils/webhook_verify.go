package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

const StandardWebhookTolerance = 5 * time.Minute

// VerifyStandardWebhook verifies a Dodo-style Standard Webhooks signature.
// Used both for founder-connected Dodo accounts and for LeakOps's own
// billing webhook, since both follow the same signing scheme.
func VerifyStandardWebhook(payload []byte, webhookID, webhookTimestamp, webhookSignature, secret string) error {
	if webhookID == "" || webhookSignature == "" || webhookTimestamp == "" {
		return errors.New("missing webhook verification headers")
	}

	timestamp, err := strconv.ParseInt(webhookTimestamp, 10, 64)
	if err != nil {
		return errors.New("invalid webhook timestamp")
	}

	eventTime := time.Unix(timestamp, 0)
	now := time.Now()
	if eventTime.Before(now.Add(-StandardWebhookTolerance)) || eventTime.After(now.Add(StandardWebhookTolerance)) {
		return errors.New("webhook timestamp outside tolerance window")
	}

	signedContent := webhookID + "." + webhookTimestamp + "." + string(payload)

	trimmedSecret := strings.TrimPrefix(secret, "whsec_")
	secretBytes, err := base64.StdEncoding.DecodeString(trimmedSecret)
	if err != nil {
		return errors.New("invalid webhook secret")
	}

	mac := hmac.New(sha256.New, secretBytes)
	if _, err := mac.Write([]byte(signedContent)); err != nil {
		return err
	}
	expected := mac.Sum(nil)

	matched := false
	for _, part := range strings.Fields(webhookSignature) {
		kv := strings.SplitN(part, ",", 2)
		if len(kv) != 2 || kv[0] != "v1" {
			continue
		}
		sigBytes, err := base64.StdEncoding.DecodeString(kv[1])
		if err != nil {
			continue
		}
		if hmac.Equal(sigBytes, expected) {
			matched = true
		}
	}

	if !matched {
		return errors.New("invalid webhook signature")
	}

	return nil
}

