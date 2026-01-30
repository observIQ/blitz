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

// generatePhone generates a random US phone number
func generatePhone(r *rand.Rand) string {
	return fmt.Sprintf("+1-%03d-%03d-%04d",
		r.Intn(900)+100,
		r.Intn(900)+100,
		r.Intn(9000)+1000)
}

// generateIntlPhone generates a random international phone number
func generateIntlPhone(r *rand.Rand) string {
	countryCodes := []string{"+44", "+49", "+33", "+81", "+86", "+91", "+61", "+55"}
	cc := countryCodes[r.Intn(len(countryCodes))]
	return fmt.Sprintf("%s-%d-%d-%04d",
		cc,
		r.Intn(900)+100,
		r.Intn(900)+100,
		r.Intn(9000)+1000)
}

// generateEmail generates a random email address
func generateEmail(r *rand.Rand) string {
	firstNames := []string{"john", "jane", "bob", "alice", "mike", "sarah", "david", "emma"}
	lastNames := []string{"smith", "jones", "wilson", "brown", "taylor", "davis", "miller", "anderson"}
	domains := []string{"gmail.com", "yahoo.com", "outlook.com", "company.com", "example.org"}
	return fmt.Sprintf("%s.%s%d@%s",
		firstNames[r.Intn(len(firstNames))],
		lastNames[r.Intn(len(lastNames))],
		r.Intn(100),
		domains[r.Intn(len(domains))])
}

// generateCreditCard generates a random credit card number (Luhn-valid format)
func generateCreditCard(r *rand.Rand) string {
	// Generate a 16-digit card number with common prefixes
	prefixes := []string{"4", "51", "52", "53", "54", "55", "34", "37"} // Visa, MC, Amex
	prefix := prefixes[r.Intn(len(prefixes))]
	remaining := 16 - len(prefix) - 1 // -1 for check digit

	number := prefix
	for i := 0; i < remaining; i++ {
		number += fmt.Sprintf("%d", r.Intn(10))
	}
	// Add a random check digit (not Luhn-valid, but looks realistic)
	number += fmt.Sprintf("%d", r.Intn(10))

	// Format with spaces
	return fmt.Sprintf("%s %s %s %s", number[0:4], number[4:8], number[8:12], number[12:16])
}

// generateDOB generates a random date of birth
func generateDOB(r *rand.Rand) string {
	// Generate DOB between 18 and 80 years ago
	year := time.Now().Year() - 18 - r.Intn(62)
	month := r.Intn(12) + 1
	day := r.Intn(28) + 1
	formats := []string{
		fmt.Sprintf("%02d/%02d/%d", month, day, year),
		fmt.Sprintf("%d-%02d-%02d", year, month, day),
		fmt.Sprintf("%02d-%02d-%d", month, day, year),
	}
	return formats[r.Intn(len(formats))]
}

// generateIPv6 generates a random IPv6 address
func generateIPv6(r *rand.Rand) string {
	return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
		r.Intn(65536), r.Intn(65536), r.Intn(65536), r.Intn(65536),
		r.Intn(65536), r.Intn(65536), r.Intn(65536), r.Intn(65536))
}

// generateMAC generates a random MAC address
func generateMAC(r *rand.Rand) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		r.Intn(256), r.Intn(256), r.Intn(256),
		r.Intn(256), r.Intn(256), r.Intn(256))
}

// generateStreetAddress generates a random US street address
func generateStreetAddress(r *rand.Rand) string {
	streetNames := []string{"Main", "Oak", "Maple", "Cedar", "Pine", "Elm", "Washington", "Park", "Lake", "Hill"}
	streetTypes := []string{"St", "Ave", "Blvd", "Dr", "Ln", "Way", "Rd", "Ct"}
	return fmt.Sprintf("%d %s %s",
		r.Intn(9999)+1,
		streetNames[r.Intn(len(streetNames))],
		streetTypes[r.Intn(len(streetTypes))])
}

// generateCityState generates a random US city and state
func generateCityState(r *rand.Rand) string {
	cities := []struct {
		city  string
		state string
	}{
		{"New York", "NY"}, {"Los Angeles", "CA"}, {"Chicago", "IL"},
		{"Houston", "TX"}, {"Phoenix", "AZ"}, {"Philadelphia", "PA"},
		{"San Antonio", "TX"}, {"San Diego", "CA"}, {"Dallas", "TX"},
		{"Austin", "TX"}, {"Seattle", "WA"}, {"Denver", "CO"},
		{"Boston", "MA"}, {"Miami", "FL"}, {"Atlanta", "GA"},
	}
	loc := cities[r.Intn(len(cities))]
	return fmt.Sprintf("%s, %s", loc.city, loc.state)
}

// generateZipCode generates a random US zip code
func generateZipCode(r *rand.Rand) string {
	if r.Float64() < 0.5 {
		return fmt.Sprintf("%05d", r.Intn(100000))
	}
	return fmt.Sprintf("%05d-%04d", r.Intn(100000), r.Intn(10000))
}

// GeneratePIIData creates structured log data for the PII log type
// Includes all common sensitive data types for comprehensive PII testing
func GeneratePIIData() (*PIILogData, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	// Generate all PII fields for every log entry
	data := &PIILogData{
		TimestampVal:   time.Now(),
		UserIDVal:      generateUserID(r),
		IBANVal:        generateIBAN(r),
		PhoneVal:       generatePhone(r),
		IntlPhoneVal:   generateIntlPhone(r),
		EmailVal:       generateEmail(r),
		CreditCardVal:  generateCreditCard(r),
		DOBVal:         generateDOB(r),
		IPv4Val:        generateIP(r),
		IPv6Val:        generateIPv6(r),
		MACAddressVal:  generateMAC(r),
		StreetAddrVal:  generateStreetAddress(r),
		CityStateVal:   generateCityState(r),
		ZipCodeVal:     generateZipCode(r),
		SSNVal:         generateSSN(r),
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
		action := actions[r.Intn(len(actions))]
		status := statuses[r.Intn(len(statuses))]
		msg := messages[r.Intn(len(messages))]

		data.LevelVal = "INFO"
		data.MessageVal = msg
		data.TypeVal = "info"
		data.ActionVal = action
		data.StatusVal = status
	}

	return data, nil
}
