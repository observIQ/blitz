package catalog

import "strings"

// SecurityIDSource (tag 22) values whose SecurityID (tag 48) carries a check
// digit.
const (
	SecurityIDSourceCUSIP = "1"
	SecurityIDSourceSEDOL = "2"
	SecurityIDSourceISIN  = "4"
)

// CUSIPCheckDigit returns the check digit for the first eight characters of a
// CUSIP (modulus 10 "double-add-double"; *, @, # count as 36-38).
func CUSIPCheckDigit(first8 string) byte {
	sum := 0
	for i := range 8 {
		v := alnumValue(first8[i])
		if i%2 == 1 {
			v *= 2
		}
		sum += v/10 + v%10
	}
	return byte('0' + (10-sum%10)%10)
}

// ValidSecurityID reports whether id is well formed for source, check digit
// included. Sources without a check digit are not judged and report true.
func ValidSecurityID(id, source string) bool {
	switch source {
	case SecurityIDSourceCUSIP:
		return len(id) == 9 && strings.Trim(id[:8], "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ*@#") == "" && id[8] == CUSIPCheckDigit(id)
	case SecurityIDSourceSEDOL:
		return len(id) == 7 && strings.Trim(id[:6], "0123456789BCDFGHJKLMNPQRSTVWXYZ") == "" && id[6] == sedolCheckDigit(id)
	case SecurityIDSourceISIN:
		return len(id) == 12 && strings.Trim(id[:2], "ABCDEFGHIJKLMNOPQRSTUVWXYZ") == "" && strings.Trim(id[2:11], "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ") == "" && id[11] == isinCheckDigit(id)
	}
	return true
}

// sedolCheckDigit weights the first six characters 1,3,1,7,3,9.
func sedolCheckDigit(id string) byte {
	weights := [6]int{1, 3, 1, 7, 3, 9}
	sum := 0
	for i, w := range weights {
		sum += alnumValue(id[i]) * w
	}
	return byte('0' + (10-sum%10)%10)
}

// isinCheckDigit expands letters to two digits (A=10) and applies Luhn.
func isinCheckDigit(id string) byte {
	var digits []int
	for i := range 11 {
		v := alnumValue(id[i])
		if v >= 10 {
			digits = append(digits, v/10)
		}
		digits = append(digits, v%10)
	}
	sum := 0
	for i := range digits {
		v := digits[len(digits)-1-i]
		if i%2 == 0 {
			v *= 2
		}
		sum += v/10 + v%10
	}
	return byte('0' + (10-sum%10)%10)
}

func alnumValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	case c == '*':
		return 36
	case c == '@':
		return 37
	case c == '#':
		return 38
	}
	return 0
}
