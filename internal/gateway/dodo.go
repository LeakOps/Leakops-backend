package gateway

import(
	"crypto/hmac"
	"crypto/sha256" 
	"encoding/base64"
	"encoding/json" 
	"errors" 
	"fmt" 
	"strconv" 
	"strings" 
	"time"
)

type DodoGateway struct {}

func NewDodoGateway() *DodoGateway {
	Type string  `json:"type"`
	Data struct {
		PaymentID		string	`json:"payment_id"`
		CustomerID		string	`json:"customer_id"`
	}
}