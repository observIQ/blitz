package logtypes

import (
	"time"
)

// LogType constants for log type selection
const (
	LogTypeDefault = "default"
	LogTypePII     = "pii"
)

// LogData is the interface for all log type data structures
// This allows generators to work with different log types in a type-safe way
type LogData interface {
	// Timestamp returns the log timestamp
	Timestamp() time.Time
	// Level returns the log severity level
	Level() string
	// Message returns the log message
	Message() string
}

// DefaultLogData represents the default log type with standard fields
type DefaultLogData struct {
	TimestampVal   time.Time
	LevelVal       string
	EnvironmentVal string
	LocationVal    string
	MessageVal     string
}

// Timestamp implements LogData interface
func (d *DefaultLogData) Timestamp() time.Time {
	return d.TimestampVal
}

// Level implements LogData interface
func (d *DefaultLogData) Level() string {
	return d.LevelVal
}

// Message implements LogData interface
func (d *DefaultLogData) Message() string {
	return d.MessageVal
}

// PIILogData represents the PII log type with banking/PII fields
// Includes all common sensitive data types for comprehensive PII testing
type PIILogData struct {
	TimestampVal time.Time
	LevelVal     string
	MessageVal   string
	EventVal     string
	DetailVal    string
	TypeVal      string
	ActionVal    string
	StatusVal    string

	// Core PII Fields
	UserIDVal      string // UUID/GUID
	SSNVal         string // Social Security Number
	IBANVal        string // International Bank Account Number
	PhoneVal       string // US Phone Number
	IntlPhoneVal   string // International Phone Number
	EmailVal       string // Email Address
	CreditCardVal  string // Credit Card Number
	DOBVal         string // Date of Birth
	IPv4Val        string // IPv4 Address
	IPv6Val        string // IPv6 Address
	MACAddressVal  string // MAC Address
	StreetAddrVal  string // US Street Address
	CityStateVal   string // US City, State
	ZipCodeVal     string // US Zip Code

	// Government IDs
	PassportVal       string // Passport Number
	DriversLicenseVal string // Driver's License Number
	NationalIDVal     string // National ID (non-US)

	// Financial
	BankAccountVal   string // Bank Account Number
	RoutingNumberVal string // ABA Routing Number
	CryptoWalletVal  string // Cryptocurrency Wallet Address

	// Healthcare
	MedicalRecordVal  string // Medical Record Number (MRN)
	HealthInsuranceVal string // Health Insurance ID

	// Vehicle
	VINVal           string // Vehicle Identification Number
	LicensePlateVal  string // License Plate Number

	// Employment/Education
	EmployeeIDVal string // Employee ID
	StudentIDVal  string // Student ID

	// Authentication/Secrets
	UsernameVal    string // Username
	PasswordHashVal string // Password Hash
	APIKeyVal      string // API Key/Token
	AWSAccessKeyVal string // AWS Access Key ID
	PrivateKeyVal  string // Private Key (partial)
	JWTTokenVal    string // JWT Token

	// Location
	GPSCoordsVal string // GPS Coordinates (lat,long)
	GeohashVal   string // Geohash

	// Personal
	FullNameVal        string // Full Name
	MothersMaidenVal   string // Mother's Maiden Name
	SecurityAnswerVal  string // Security Question Answer
}

// Timestamp implements LogData interface
func (p *PIILogData) Timestamp() time.Time {
	return p.TimestampVal
}

// Level implements LogData interface
func (p *PIILogData) Level() string {
	return p.LevelVal
}

// Message implements LogData interface
func (p *PIILogData) Message() string {
	return p.MessageVal
}
