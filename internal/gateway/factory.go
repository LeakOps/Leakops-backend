package gateway

import(
	"errors"
	"strings"
)

// GetGateway returns the PaymentGateway implementation for a gateway type
// string coming from the URL path (e.g. "stripe", "dodo").
//
// Keeping this as a small factory (instead of, say, a map initialized at
// package level) means each call gets a fresh, stateless gateway struct —
// StripeGateway and DodoGateway hold no fields, so this has no real cost, and
// it avoids any shared/global state across requests (the same principle
// RetryPayment follows by building a per-request Stripe client).
func GetGateway(gatewayType string) (PaymentGateway, error) {
	switch strings.ToLower(strings.TrimSpace(gatewayType)) {
	case string(GatewayTypeStripe):
		return NewStripeGateway(), nil
	case string(GatewayTypeDodo):
		return NewDodoGateway(), nil
	default:
		return nil, errors.New("unsupported gateway type")
	}
}

// GatewayTypeStripe / GatewayTypeDodo mirror models.GatewayStripe /
// models.GatewayDodo, but are declared here (in the gateway package) so this
// file doesn't need to import the models package just for two string
// constants — avoids a potential import cycle if models ever imports gateway.
const (
	GatewayTypeStripe = "stripe"
	GatewayTypeDodo = "dodo"
)