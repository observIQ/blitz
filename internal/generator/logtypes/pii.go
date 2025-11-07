package logtypes

import (
	"fmt"
	"math/rand"
	"time"
)

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

// GeneratePIIData creates structured log data for the PII log type
func GeneratePIIData() (*PIILogData, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	userID := generateUserID(r)
	iban := generateIBAN(r)
	phone := generatePhone(r)

	data := &PIILogData{
		TimestampVal: time.Now(),
		UserIDVal:    userID,
		IBANVal:      iban,
		PhoneVal:     phone,
	}

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

		data.LevelVal = "ERROR"
		data.MessageVal = errorMsg
		data.EventVal = errorMsg
		data.DetailVal = detail
	} else {
		ssn := generateSSN(r)
		action := actions[r.Intn(len(actions))]
		status := statuses[r.Intn(len(statuses))]
		msg := messages[r.Intn(len(messages))]

		data.LevelVal = "INFO"
		data.MessageVal = msg
		data.TypeVal = "info"
		data.ActionVal = action
		data.StatusVal = status
		data.SSNVal = ssn
	}

	return data, nil
}

