package logtypes

import (
	"fmt"
	"math/rand"
	"time"

	jsonlib "github.com/goccy/go-json"
	"github.com/observiq/blitz/output"
)

// piiLog represents a PII log entry with banking-related fields
type piiLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Event     string    `json:"event,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	Type      string    `json:"type,omitempty"`
	Action    string    `json:"action,omitempty"`
	Status    string    `json:"status,omitempty"`
	UserID    string    `json:"user_id"`
	SSN       string    `json:"ssn,omitempty"`
	IBAN      string    `json:"iban"`
	Phone     string    `json:"phone"`
}

var (
	actions = []string{
		"processed transaction",
		"approved loan application",
		"updated account information",
		"verified customer identity",
		"completed wire transfer",
		"reviewed credit application",
		"processed payment",
		"updated security settings",
	}

	statuses = []string{
		"successful",
		"pending review",
		"requires additional verification",
		"completed",
		"approved",
		"rejected",
	}

	messages = []string{
		"Customer service request completed",
		"Account activity processed",
		"Security verification completed",
		"Transaction authorization completed",
		"Account update processed",
		"Customer verification completed",
		"Payment processing completed",
		"Account settings updated",
	}

	errorMessages = []string{
		"Invalid SSN format provided",
		"Database connection timeout",
		"Upstream service unavailable",
		"Invalid transaction amount",
		"Rate limit exceeded",
		"Authentication failed",
		"Account locked due to suspicious activity",
		"Invalid routing number",
		"Insufficient funds",
		"Transaction declined by fraud detection",
	}

	errorDetails = []string{
		"Database connection failed after 3 retries",
		"Customer provided malformed SSN: %s",
		"Payment processing service returned 503",
		"Transaction amount exceeds daily limit",
		"Too many requests from IP: %s",
		"Invalid security credentials",
		"Multiple failed login attempts detected",
		"Invalid ACH routing number format",
		"Account balance insufficient for transaction",
		"Fraud score threshold exceeded",
	}
)

const errorProbability = 0.3

// generateSSN generates a random SSN in XXX-XX-XXXX format
func generateSSN(r *rand.Rand) string {
	return fmt.Sprintf("%03d-%02d-%04d",
		r.Intn(900)+100,
		r.Intn(90)+10,
		r.Intn(9000)+1000)
}

// generateIP generates a random IP address
func generateIP(r *rand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		r.Intn(256),
		r.Intn(256),
		r.Intn(256),
		r.Intn(256))
}

// generateUserID generates a random user ID
func generateUserID(r *rand.Rand) string {
	hi := r.Uint64()
	lo := r.Uint64()
	return fmt.Sprintf("%016x-%016x", hi, lo)
}

// generateIBAN generates a random IBAN
func generateIBAN(r *rand.Rand) string {
	return fmt.Sprintf("US%02d%04d%04d%012d",
		r.Intn(100),
		r.Intn(10000),
		r.Intn(10000),
		r.Intn(1000000000000))
}

// generatePhone generates a random phone number
func generatePhone(r *rand.Rand) string {
	return fmt.Sprintf("+1-%03d-%03d-%04d",
		r.Intn(900)+100,
		r.Intn(900)+100,
		r.Intn(9000)+1000)
}

// GeneratePIILog creates a random log entry with PII fields
func GeneratePIILog() (output.LogRecord, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	userID := generateUserID(r)
	iban := generateIBAN(r)
	phone := generatePhone(r)

	var log piiLog
	log.Timestamp = time.Now()
	log.UserID = userID
	log.IBAN = iban
	log.Phone = phone

	if r.Float64() < errorProbability {
		errorIdx := r.Intn(len(errorMessages))
		errorMsg := errorMessages[errorIdx]
		errorDetail := errorDetails[errorIdx]

		var detail string
		switch errorIdx {
		case 1:
			detail = fmt.Sprintf(errorDetail, generateSSN(r))
		case 4:
			detail = fmt.Sprintf(errorDetail, generateIP(r))
		default:
			detail = errorDetail
		}

		log.Level = "ERROR"
		log.Message = errorMsg
		log.Event = errorMsg
		log.Detail = detail
	} else {
		ssn := generateSSN(r)
		action := actions[r.Intn(len(actions))]
		status := statuses[r.Intn(len(statuses))]
		msg := messages[r.Intn(len(messages))]

		log.Level = "INFO"
		log.Message = msg
		log.Type = "info"
		log.Action = action
		log.Status = status
		log.SSN = ssn
	}

	b, err := jsonlib.Marshal(log)
	if err != nil {
		return output.LogRecord{}, fmt.Errorf("marshal JSON log: %w", err)
	}

	return output.LogRecord{
		Message: string(b),
		ParseFunc: func(message string) (map[string]any, error) {
			var parsed map[string]any
			if err := jsonlib.Unmarshal([]byte(message), &parsed); err != nil {
				return nil, fmt.Errorf("unmarshal JSON log: %w", err)
			}
			return parsed, nil
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: log.Timestamp,
			Severity:  log.Level,
		},
	}, nil
}
