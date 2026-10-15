// Package waf holds the attack scenarios shared by the two F5 WAF products,
// BIG-IP ASM and NGINX App Protect. Both run the same enforcement engine, so a
// scenario's request, violations, attack types, and signatures read the same
// in either product's log; each product renders them in its own field set.
//
// Every scenario is taken from a real App Protect security-log line in F5's
// NGINX Gateway Fabric lab (f5devcentral/NGINX-Gateway-Fabric-Lab,
// labs/11.waf/README.md at 00c6cae). The JSON and XML payloads are verbatim.
package waf

// Scenario is one WAF detection: the request that triggered it and what the
// engine reported about it.
type Scenario struct {
	// Method, Path, and Query form the request line; Query may be empty.
	Method, Path, Query string
	// HostHeader and UserAgent are the request's Host and User-Agent headers.
	HostHeader, UserAgent string
	// VSName is App Protect's location identifier for the request.
	VSName string
	// Policy is the enforcing policy name.
	Policy string
	// Status is the request_status: "blocked" or "alerted".
	Status string
	// ResponseCode is the logged response code (0 when blocked).
	ResponseCode int
	// Outcome, OutcomeReason, and EnforcementAction are App Protect's
	// verdict fields.
	Outcome, OutcomeReason, EnforcementAction string
	// AttackTypes, Violations, SigIDs, SigNames, SigSetNames, and SigCVEs are
	// the comma-joined lists as logged ("N/A" when empty).
	AttackTypes, Violations, SigIDs, SigNames, SigSetNames, SigCVEs string
	// Severity and Rating are the maximum violation severity and the 0-5
	// violation rating.
	Severity string
	Rating   int
	// BotSignature, BotCategory, and ClientClass classify the client.
	BotSignature, BotCategory, ClientClass string
	// JSONViolations and JSONEnforcement are json_log's "violations" array
	// and top-level "enforcementState" object.
	JSONViolations, JSONEnforcement string
	// ViolationDetails is the violation_details XML.
	ViolationDetails string
}

// Target returns the request target: the path plus "?query" when present.
func (s Scenario) Target() string {
	if s.Query == "" {
		return s.Path
	}
	return s.Path + "?" + s.Query
}

// RawRequest returns the request as the engine saw it, CRLF line endings
// included.
func (s Scenario) RawRequest() string {
	return s.Method + " " + s.Target() + " HTTP/1.1\r\nHost: " + s.HostHeader +
		"\r\nUser-Agent: " + s.UserAgent + "\r\nAccept: */*\r\n\r\n"
}

// Scenarios is every sourced scenario.
// Two scenarios, both from one lab capture; add more as further
// real App Protect lines are sourced.
var Scenarios = []Scenario{XSSParameter, DataGuardLeak}

// XSSParameter is a blocked reflected-XSS attempt in a query parameter.
var XSSParameter = Scenario{
	Method: "GET", Path: "/customers", Query: "x=</script>",
	HostHeader: "cafe.example.com:31628", UserAgent: "curl/8.5.0",
	VSName: "56-cafe.example.com:3-/customers", Policy: "attack-signatures-blocking",
	Status: "blocked", ResponseCode: 0,
	Outcome: "REJECTED", OutcomeReason: "SECURITY_WAF_VIOLATION", EnforcementAction: "block",
	AttackTypes: "Non-browser Client,Abuse of Functionality,Cross Site Scripting (XSS),Other Application Activity",
	Violations:  "Illegal meta character in value,Attack signature detected,Violation Rating Threat detected,Bot Client Detected",
	SigIDs:      "200001475,200000098",
	SigNames:    "XSS script tag end (Parameter) (2),XSS script tag (Parameter)",
	SigSetNames: "{High Accuracy Signatures;Cross Site Scripting Signatures;All Signatures},{High Accuracy Signatures;Cross Site Scripting Signatures;All Signatures}",
	SigCVEs:     "N/A,N/A",
	Severity:    "Critical", Rating: 5,
	BotSignature: "curl", BotCategory: "HTTP Library", ClientClass: "Untrusted Bot",
	JSONViolations: xssParameterJSONViolations, JSONEnforcement: xssParameterJSONEnforcement,
	ViolationDetails: xssParameterViolationDetails,
}

// DataGuardLeak is an alerted Data Guard hit: the response leaked a card
// number and an SSN.
var DataGuardLeak = Scenario{
	Method: "GET", Path: "/customers",
	HostHeader: "cafe.example.com:31628", UserAgent: "curl/8.5.0",
	VSName: "56-cafe.example.com:3-/customers", Policy: "dataguard-blocking",
	Status: "alerted", ResponseCode: 200,
	Outcome: "PASSED", OutcomeReason: "SECURITY_WAF_FLAGGED", EnforcementAction: "none",
	AttackTypes: "Non-browser Client,Information Leakage",
	Violations:  "Data Guard: Information leakage detected,Bot Client Detected",
	SigIDs:      "N/A", SigNames: "N/A", SigSetNames: "N/A", SigCVEs: "N/A",
	Severity: "Critical", Rating: 5,
	BotSignature: "curl", BotCategory: "HTTP Library", ClientClass: "Untrusted Bot",
	JSONViolations: dataGuardLeakJSONViolations, JSONEnforcement: dataGuardLeakJSONEnforcement,
	ViolationDetails: dataGuardLeakViolationDetails,
}

// xssParameterJSONViolations is the json_log "violations" array, verbatim.
const xssParameterJSONViolations = `[{"enforcementState":{"isBlocked":true,"isAlarmed":true,"isInStaging":false,"isLearned":false,"isLikelyFalsePositive":false,"attackType":[{"name":"Cross Site Scripting (XSS)"}]},"violation":{"name":"VIOL_ATTACK_SIGNATURE"},"signature":{"name":"XSS script tag end (Parameter) (2)","signatureId":200001475,"accuracy":"high","risk":"high","hasCve":false,"stagingCertificationDatetime":"1970-01-01T00:00:00Z","lastUpdateTime":"2025-01-08T16:57:22Z"},"snippet":{"buffer":"eD08L3NjcmlwdD4=","offset":4,"length":7},"policyEntity":{"parameters":[{"name":"*","level":"global","type":"wildcard"}]},"observedEntity":{"name":"eA==","value":"PC9zY3JpcHQ+","location":"query"}},{"enforcementState":{"isBlocked":true,"isAlarmed":true,"isInStaging":false,"isLearned":false,"isLikelyFalsePositive":false,"attackType":[{"name":"Cross Site Scripting (XSS)"}]},"violation":{"name":"VIOL_ATTACK_SIGNATURE"},"signature":{"name":"XSS script tag (Parameter)","signatureId":200000098,"accuracy":"high","risk":"high","hasCve":false,"stagingCertificationDatetime":"1970-01-01T00:00:00Z","lastUpdateTime":"2023-11-02T19:36:54Z"},"snippet":{"buffer":"eD08L3NjcmlwdD4=","offset":2,"length":8},"policyEntity":{"parameters":[{"name":"*","level":"global","type":"wildcard"}]},"observedEntity":{"name":"eA==","value":"PC9zY3JpcHQ+","location":"query"}},{"enforcementState":{"isBlocked":false,"isAlarmed":true,"isLearned":false,"attackType":[{"name":"Abuse of Functionality"}]},"violation":{"name":"VIOL_PARAMETER_VALUE_METACHAR"},"policyEntity":{"parameters":[{"name":"*","level":"global","type":"wildcard"}]},"observedEntity":{"name":"eA==","value":"PC9zY3JpcHQ+","location":"query"},"metachar":"0x3c","charsetType":"parameter-value"},{"enforcementState":{"isBlocked":false,"isAlarmed":true,"isLearned":false,"attackType":[{"name":"Abuse of Functionality"}]},"violation":{"name":"VIOL_PARAMETER_VALUE_METACHAR"},"policyEntity":{"parameters":[{"name":"*","level":"global","type":"wildcard"}]},"observedEntity":{"name":"eA==","value":"PC9zY3JpcHQ+","location":"query"},"metachar":"0x3e","charsetType":"parameter-value"},{"enforcementState":{"isBlocked":false,"isAlarmed":true,"isLearned":true,"attackType":[{"name":"Non-browser Client"}]},"violation":{"name":"VIOL_BOT_CLIENT"},"botSignature":{"name":"curl","category":"HTTP Library","botClass":"Untrusted Bot"}},{"enforcementState":{"isBlocked":true,"isAlarmed":true,"attackType":[{"name":"Other Application Activity"}]},"violation":{"name":"VIOL_RATING_THREAT"}}]`

// xssParameterJSONEnforcement is the json_log top-level "enforcementState", verbatim.
const xssParameterJSONEnforcement = `{"isBlocked":true,"isAlarmed":true,"rating":5,"attackType":[{"name":"Non-browser Client"},{"name":"Abuse of Functionality"},{"name":"Cross Site Scripting (XSS)"},{"name":"Other Application Activity"}],"ratingIncludingViolationsInStaging":5,"stagingCertificationDatetime":"1970-01-01T00:00:00Z"}`

// xssParameterViolationDetails is the violation_details XML, verbatim.
const xssParameterViolationDetails = `<?xml version='1.0' encoding='UTF-8'?><BAD_MSG><violation_masks><block>414000000200c00-3a03030c30000072-8000000000000000-0</block><alarm>475f0ffcbbd0fea-befbf35cb000007e-f400000000000000-0</alarm><learn>0-0-0-0</learn><staging>0-0-0-0</staging></violation_masks><request-violations><violation><viol_index>42</viol_index><viol_name>VIOL_ATTACK_SIGNATURE</viol_name><context>parameter</context><parameter_data><value_error/><enforcement_level>global</enforcement_level><name>eA==</name><auto_detected_type>alpha-numeric</auto_detected_type><value>PC9zY3JpcHQ+</value><location>query</location><expected_location></expected_location><is_base64_decoded>false</is_base64_decoded><param_name_pattern>*</param_name_pattern><staging>0</staging></parameter_data><staging>0</staging><sig_data><sig_id>200001475</sig_id><blocking_mask>3</blocking_mask><kw_data><buffer>eD08L3NjcmlwdD4=</buffer><offset>4</offset><length>7</length></kw_data></sig_data><sig_data><sig_id>200000098</sig_id><blocking_mask>3</blocking_mask><kw_data><buffer>eD08L3NjcmlwdD4=</buffer><offset>2</offset><length>8</length></kw_data></sig_data></violation><violation><viol_index>24</viol_index><viol_name>VIOL_PARAMETER_VALUE_METACHAR</viol_name><parameter_data><value_error/><enforcement_level>global</enforcement_level><name>eA==</name><auto_detected_type>alpha-numeric</auto_detected_type><value>PC9zY3JpcHQ+</value><location>query</location><expected_location></expected_location><is_base64_decoded>false</is_base64_decoded></parameter_data><wildcard_entity>*</wildcard_entity><staging>0</staging><language_type>4</language_type><metachar_index>60</metachar_index><metachar_index>62</metachar_index></violation><violation><viol_index>122</viol_index><viol_name>VIOL_BOT_CLIENT</viol_name></violation><violation><viol_index>93</viol_index><viol_name>VIOL_RATING_THREAT</viol_name></violation></request-violations></BAD_MSG>`

// dataGuardLeakJSONViolations is the json_log "violations" array, verbatim.
const dataGuardLeakJSONViolations = `[{"enforcementState":{"isBlocked":false,"isAlarmed":true,"isLearned":true,"attackType":[{"name":"Non-browser Client"}]},"violation":{"name":"VIOL_BOT_CLIENT"},"botSignature":{"name":"curl","category":"HTTP Library","botClass":"Untrusted Bot"}},{"enforcementState":{"isBlocked":false,"isAlarmed":true,"attackType":[{"name":"Information Leakage"}]},"violation":{"name":"VIOL_DATA_GUARD"}}]`

// dataGuardLeakJSONEnforcement is the json_log top-level "enforcementState", verbatim.
const dataGuardLeakJSONEnforcement = `{"isBlocked":false,"isAlarmed":false,"rating":5,"attackType":[{"name":"Non-browser Client"},{"name":"Information Leakage"}],"ratingIncludingViolationsInStaging":5,"stagingCertificationDatetime":"1970-01-01T00:00:00Z"}`

// dataGuardLeakViolationDetails is the violation_details XML, verbatim.
const dataGuardLeakViolationDetails = `<?xml version='1.0' encoding='UTF-8'?><BAD_MSG><violation_masks><block>414000000000c00-3a03030c30000072-8000000000000000-0</block><alarm>2475f0ffcb9d0fea-befbf35cb000007e-f400000000000000-0</alarm><learn>0-0-0-0</learn><staging>0-0-0-0</staging></violation_masks><request-violations><violation><viol_index>122</viol_index><viol_name>VIOL_BOT_CLIENT</viol_name></violation></request-violations><response_violations><violation><viol_index>2</viol_index><viol_name>VIOL_DATA_GUARD</viol_name><leakage_type>illegal_pattern</leakage_type><pattern_type>CCN</pattern_type><cfg_target>L2N1c3RvbWVycw==</cfg_target><string>YXJkOiAqKioqKgpTU046</string></violation><violation><viol_index>2</viol_index><viol_name>VIOL_DATA_GUARD</viol_name><leakage_type>illegal_pattern</leakage_type><pattern_type>SSN</pattern_type><cfg_target>L2N1c3RvbWVycw==</cfg_target><string>U1NOOiAqKioqKgoK</string></violation></response_violations></BAD_MSG>`
