package logtypes

import (
	"fmt"
	"math/rand"
	"strings"
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
	for range remaining {
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

// generatePassport generates a random passport number
func generatePassport(r *rand.Rand) string {
	// US passport format: 1 letter + 8 digits, or 9 digits
	if r.Float64() < 0.5 {
		return fmt.Sprintf("%c%08d", 'A'+r.Intn(26), r.Intn(100000000))
	}
	return fmt.Sprintf("%09d", r.Intn(1000000000))
}

// generateDriversLicense generates a random US driver's license number
func generateDriversLicense(r *rand.Rand) string {
	// Various state formats
	states := []string{"CA", "NY", "TX", "FL", "IL"}
	state := states[r.Intn(len(states))]
	switch state {
	case "CA":
		return fmt.Sprintf("%c%07d", 'A'+r.Intn(26), r.Intn(10000000))
	case "NY":
		return fmt.Sprintf("%03d-%03d-%03d", r.Intn(1000), r.Intn(1000), r.Intn(1000))
	default:
		return fmt.Sprintf("%s%08d", state, r.Intn(100000000))
	}
}

// generateNationalID generates a random national ID (non-US)
func generateNationalID(r *rand.Rand) string {
	// Various formats: UK NI, Canadian SIN, etc.
	formats := []string{
		fmt.Sprintf("AB%06dC", r.Intn(1000000)),                                               // UK National Insurance
		fmt.Sprintf("%03d-%03d-%03d", r.Intn(1000), r.Intn(1000), r.Intn(1000)),               // Canadian SIN
		fmt.Sprintf("%02d%02d%02d-%05d", r.Intn(100), r.Intn(13), r.Intn(32), r.Intn(100000)), // Various EU
	}
	return formats[r.Intn(len(formats))]
}

// generateBankAccount generates a random bank account number
func generateBankAccount(r *rand.Rand) string {
	// 8-17 digits typical
	length := 8 + r.Intn(10)
	var account strings.Builder
	for range length {
		account.WriteString(fmt.Sprintf("%d", r.Intn(10)))
	}
	return account.String()
}

// generateRoutingNumber generates a random ABA routing number
func generateRoutingNumber(r *rand.Rand) string {
	return fmt.Sprintf("%09d", r.Intn(1000000000))
}

// generateCryptoWallet generates a random cryptocurrency wallet address
func generateCryptoWallet(r *rand.Rand) string {
	// Bitcoin or Ethereum format
	if r.Float64() < 0.5 {
		// Bitcoin (starts with 1, 3, or bc1)
		prefixes := []string{"1", "3", "bc1q"}
		prefix := prefixes[r.Intn(len(prefixes))]
		chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		var addr strings.Builder
		addr.WriteString(prefix)
		length := 26 + r.Intn(10)
		for range length {
			addr.WriteString(string(chars[r.Intn(len(chars))]))
		}
		return addr.String()
	}
	// Ethereum (starts with 0x, 40 hex chars)
	return fmt.Sprintf("0x%040x", r.Uint64())
}

// generateMedicalRecord generates a random Medical Record Number
func generateMedicalRecord(r *rand.Rand) string {
	return fmt.Sprintf("MRN-%08d", r.Intn(100000000))
}

// generateHealthInsurance generates a random health insurance ID
func generateHealthInsurance(r *rand.Rand) string {
	// Medicare-style or private insurance
	if r.Float64() < 0.5 {
		return fmt.Sprintf("%d%c%c%c%d", r.Intn(10), 'A'+r.Intn(26), 'A'+r.Intn(26), 'A'+r.Intn(26), r.Intn(10))
	}
	return fmt.Sprintf("%s%09d", []string{"BCBS", "UHC", "AETNA", "CIGNA"}[r.Intn(4)], r.Intn(1000000000))
}

// generateVIN generates a random Vehicle Identification Number
func generateVIN(r *rand.Rand) string {
	// VIN is 17 characters, excludes I, O, Q
	chars := "ABCDEFGHJKLMNPRSTUVWXYZ0123456789"
	var vin strings.Builder
	for range 17 {
		vin.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return vin.String()
}

// generateLicensePlate generates a random license plate
func generateLicensePlate(r *rand.Rand) string {
	formats := []string{
		fmt.Sprintf("%c%c%c-%d%d%d%d", 'A'+r.Intn(26), 'A'+r.Intn(26), 'A'+r.Intn(26), r.Intn(10), r.Intn(10), r.Intn(10), r.Intn(10)),
		fmt.Sprintf("%d%c%c%c%d%d%d", r.Intn(10), 'A'+r.Intn(26), 'A'+r.Intn(26), 'A'+r.Intn(26), r.Intn(10), r.Intn(10), r.Intn(10)),
		fmt.Sprintf("%c%c%c %d%d%d%d", 'A'+r.Intn(26), 'A'+r.Intn(26), 'A'+r.Intn(26), r.Intn(10), r.Intn(10), r.Intn(10), r.Intn(10)),
	}
	return formats[r.Intn(len(formats))]
}

// generateEmployeeID generates a random employee ID
func generateEmployeeID(r *rand.Rand) string {
	return fmt.Sprintf("EMP%06d", r.Intn(1000000))
}

// generateStudentID generates a random student ID
func generateStudentID(r *rand.Rand) string {
	return fmt.Sprintf("STU%09d", r.Intn(1000000000))
}

// generateUsername generates a random username
func generateUsername(r *rand.Rand) string {
	adjectives := []string{"happy", "quick", "clever", "bright", "swift", "cool", "super", "mega"}
	nouns := []string{"user", "coder", "dev", "ninja", "guru", "master", "wizard", "hero"}
	return fmt.Sprintf("%s_%s%d", adjectives[r.Intn(len(adjectives))], nouns[r.Intn(len(nouns))], r.Intn(1000))
}

// generatePasswordHash generates a random password hash
func generatePasswordHash(r *rand.Rand) string {
	// Looks like bcrypt hash
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789./"
	var hash strings.Builder
	hash.WriteString("$2a$10$")
	for range 53 {
		hash.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return hash.String()
}

// generateAPIKey generates a random API key
func generateAPIKey(r *rand.Rand) string {
	prefixes := []string{"apikey_", "secret_", "token_", "key_", "access_key_"}
	prefix := prefixes[r.Intn(len(prefixes))]
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var key strings.Builder
	key.WriteString(prefix)
	for range 32 {
		key.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return key.String()
}

// generateAWSAccessKey generates a random AWS Access Key ID
func generateAWSAccessKey(r *rand.Rand) string {
	// AWS Access Key IDs start with AKIA, ABIA, ACCA, or ASIA
	prefixes := []string{"AKIA", "ABIA", "ACCA", "ASIA"}
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var key strings.Builder
	key.WriteString(prefixes[r.Intn(len(prefixes))])
	for range 16 {
		key.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return key.String()
}

// generatePrivateKey generates a partial private key representation
func generatePrivateKey(r *rand.Rand) string {
	return fmt.Sprintf("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA%s...[REDACTED]\n-----END RSA PRIVATE KEY-----",
		fmt.Sprintf("%016x", r.Uint64()))
}

// generateJWTToken generates a random JWT token
func generateJWTToken(r *rand.Rand) string {
	// Generate fake but realistic-looking JWT
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	var payload strings.Builder
	for range 36 {
		payload.WriteString(string(chars[r.Intn(len(chars))]))
	}
	var signature strings.Builder
	for range 43 {
		signature.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return fmt.Sprintf("%s.%s.%s", header, payload.String(), signature.String())
}

// generateGPSCoords generates random GPS coordinates
func generateGPSCoords(r *rand.Rand) string {
	lat := -90.0 + r.Float64()*180.0
	long := -180.0 + r.Float64()*360.0
	return fmt.Sprintf("%.6f,%.6f", lat, long)
}

// generateGeohash generates a random geohash
func generateGeohash(r *rand.Rand) string {
	chars := "0123456789bcdefghjkmnpqrstuvwxyz"
	var hash strings.Builder
	length := 6 + r.Intn(6) // 6-12 characters
	for range length {
		hash.WriteString(string(chars[r.Intn(len(chars))]))
	}
	return hash.String()
}

// generateFullName generates a random full name
func generateFullName(r *rand.Rand) string {
	firstNames := []string{"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda", "William", "Elizabeth"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez"}
	return fmt.Sprintf("%s %s", firstNames[r.Intn(len(firstNames))], lastNames[r.Intn(len(lastNames))])
}

// generateMothersMaiden generates a random mother's maiden name
func generateMothersMaiden(r *rand.Rand) string {
	names := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore"}
	return names[r.Intn(len(names))]
}

// generateSecurityAnswer generates a random security question answer
func generateSecurityAnswer(r *rand.Rand) string {
	answers := []string{"Fluffy", "Oak Street Elementary", "Blue", "Toyota Camry", "New York", "Pizza", "Rover", "Springfield", "1995", "Jennifer"}
	return answers[r.Intn(len(answers))]
}

// GeneratePIIData creates structured log data for the PII log type
// Includes all common sensitive data types for comprehensive PII testing
func GeneratePIIData() (*PIILogData, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	// Generate all PII fields for every log entry
	data := &PIILogData{
		TimestampVal: time.Now(),

		// Core PII
		UserIDVal:     generateUserID(r),
		SSNVal:        generateSSN(r),
		IBANVal:       generateIBAN(r),
		PhoneVal:      generatePhone(r),
		IntlPhoneVal:  generateIntlPhone(r),
		EmailVal:      generateEmail(r),
		CreditCardVal: generateCreditCard(r),
		DOBVal:        generateDOB(r),
		IPv4Val:       generateIP(r),
		IPv6Val:       generateIPv6(r),
		MACAddressVal: generateMAC(r),
		StreetAddrVal: generateStreetAddress(r),
		CityStateVal:  generateCityState(r),
		ZipCodeVal:    generateZipCode(r),

		// Government IDs
		PassportVal:       generatePassport(r),
		DriversLicenseVal: generateDriversLicense(r),
		NationalIDVal:     generateNationalID(r),

		// Financial
		BankAccountVal:   generateBankAccount(r),
		RoutingNumberVal: generateRoutingNumber(r),
		CryptoWalletVal:  generateCryptoWallet(r),

		// Healthcare
		MedicalRecordVal:   generateMedicalRecord(r),
		HealthInsuranceVal: generateHealthInsurance(r),

		// Vehicle
		VINVal:          generateVIN(r),
		LicensePlateVal: generateLicensePlate(r),

		// Employment/Education
		EmployeeIDVal: generateEmployeeID(r),
		StudentIDVal:  generateStudentID(r),

		// Authentication/Secrets
		UsernameVal:     generateUsername(r),
		PasswordHashVal: generatePasswordHash(r),
		APIKeyVal:       generateAPIKey(r),
		AWSAccessKeyVal: generateAWSAccessKey(r),
		PrivateKeyVal:   generatePrivateKey(r),
		JWTTokenVal:     generateJWTToken(r),

		// Location
		GPSCoordsVal: generateGPSCoords(r),
		GeohashVal:   generateGeohash(r),

		// Personal
		FullNameVal:       generateFullName(r),
		MothersMaidenVal:  generateMothersMaiden(r),
		SecurityAnswerVal: generateSecurityAnswer(r),
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
