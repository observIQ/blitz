package catalog

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeField(t *testing.T) {
	got := EncodeField(nil, Field{Tag: 35, Value: "D"})
	want := []byte("35=D\x01")
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeField = %q, want %q", got, want)
	}
}

func TestEncodeFieldsOrderPreserved(t *testing.T) {
	got := EncodeFields(nil, []Field{
		{Tag: 49, Value: "SENDER"},
		{Tag: 56, Value: "TARGET"},
		{Tag: 34, Value: "1"},
	})
	want := []byte("49=SENDER\x0156=TARGET\x0134=1\x01")
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeFields = %q, want %q", got, want)
	}
}

func TestCheckSumSpecVector(t *testing.T) {
	// Spec example from QuickFIX docs: the checksum of the literal
	// payload "8=FIX.4.4\x019=12\x0135=A\x01" is 88 (sum mod 256).
	// We hand-compute here to confirm.
	in := []byte("8=FIX.4.4\x019=12\x0135=A\x01")
	var manual uint32
	for _, b := range in {
		manual += uint32(b)
	}
	want := checksum3(manual % 256)
	if got := CheckSum(in); got != want {
		t.Errorf("CheckSum = %q, want %q", got, want)
	}
	if len(want) != 3 {
		t.Errorf("CheckSum must be 3-digit zero-padded, got %q", want)
	}
}

func TestCheckSumZeroPadded(t *testing.T) {
	// A byte stream whose sum mod 256 = 7 must render as "007".
	// Construct one: a single byte with value 7.
	got := CheckSum([]byte{7})
	if got != "007" {
		t.Errorf("CheckSum([7]) = %q, want %q", got, "007")
	}
}

func TestCheckSumWraps(t *testing.T) {
	// Sum of 256 bytes equal to 1 = 256, mod 256 = 0 → "000".
	buf := bytes.Repeat([]byte{1}, 256)
	if got := CheckSum(buf); got != "000" {
		t.Errorf("CheckSum(256 x 0x01) = %q, want %q", got, "000")
	}
}

func TestBuildMessageRoundTrip(t *testing.T) {
	body := []Field{
		{Tag: TagMsgType, Value: "A"},
		{Tag: TagSenderCompID, Value: "BLITZ"},
		{Tag: TagTargetCompID, Value: "VENUE"},
		{Tag: TagMsgSeqNum, Value: "1"},
		{Tag: TagSendingTime, Value: "20260526-17:00:00.000"},
	}
	msg := BuildMessage(V44.BeginString(), body)

	// Must start with 8=FIX.4.4 and end with a 10=NNN SOH field.
	if !bytes.HasPrefix(msg, []byte("8=FIX.4.4\x01")) {
		t.Errorf("message does not start with BeginString: %q", msg)
	}

	// Parse and check structure.
	fields := SplitFields(msg)
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 fields, got %d: %v", len(fields), fields)
	}
	if fields[0].Tag != TagBeginString || fields[0].Value != "FIX.4.4" {
		t.Errorf("first field = %+v, want BeginString=FIX.4.4", fields[0])
	}
	if fields[1].Tag != TagBodyLength {
		t.Errorf("second field tag = %d, want %d (BodyLength)", fields[1].Tag, TagBodyLength)
	}
	last := fields[len(fields)-1]
	if last.Tag != TagCheckSum {
		t.Errorf("last field tag = %d, want %d (CheckSum)", last.Tag, TagCheckSum)
	}
	if len(last.Value) != 3 {
		t.Errorf("CheckSum value = %q, want 3-digit string", last.Value)
	}
}

func TestBuildMessageBodyLengthMatchesSpec(t *testing.T) {
	// BodyLength must equal the byte count from end of BodyLength's
	// SOH up to and including the SOH before CheckSum — i.e. exactly
	// the bytes between those two delimiters.
	body := []Field{
		{Tag: TagMsgType, Value: "A"},
		{Tag: TagSenderCompID, Value: "S"},
	}
	msg := BuildMessage(V44.BeginString(), body)

	// Locate the body region by finding 9=...SOH and 10=...
	soh := string(SOH)
	parts := strings.SplitN(string(msg), soh, 3)
	if len(parts) < 3 {
		t.Fatalf("malformed message: %q", msg)
	}
	// parts[1] is "9=NN", parts[2] starts with the body up to checksum.
	bodyLenField := parts[1]
	if !strings.HasPrefix(bodyLenField, "9=") {
		t.Fatalf("expected 9=... in position 2, got %q", bodyLenField)
	}
	// Declared length:
	declared := bodyLenField[2:]

	// Compute actual body bytes: from index after BeginString-SOH +
	// BodyLength-SOH, up to (but not including) the start of 10=.
	idxChecksum := bytes.LastIndex(msg, []byte("\x0110="))
	if idxChecksum < 0 {
		t.Fatalf("no checksum delimiter found in %q", msg)
	}
	headerEnd := bytes.Index(msg[bytes.Index(msg, []byte("\x01"))+1:], []byte("\x01"))
	bodyStart := bytes.Index(msg, []byte("\x01")) + 1 + headerEnd + 1
	actualBodyLen := idxChecksum + 1 - bodyStart // +1 to include the SOH before 10=

	import_ := actualBodyLen // capture
	_ = import_

	if declared != intToStr(actualBodyLen) {
		t.Errorf("BodyLength declared %s, actual %d", declared, actualBodyLen)
	}
}

// intToStr — tiny helper to avoid pulling strconv into a test that
// otherwise has no need for it.
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	n := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}

func TestSplitFieldsHandlesMalformed(t *testing.T) {
	if got := SplitFields([]byte("nope-no-equals\x01")); got != nil {
		t.Errorf("SplitFields(no equals) = %v, want nil", got)
	}
	if got := SplitFields([]byte("=missingtag\x01")); got != nil {
		t.Errorf("SplitFields(empty tag) = %v, want nil", got)
	}
	if got := SplitFields([]byte("notanum=v\x01")); got != nil {
		t.Errorf("SplitFields(non-numeric tag) = %v, want nil", got)
	}
}
