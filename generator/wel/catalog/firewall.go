package catalog

import (
	"fmt"
	"math/rand"
)

const firewallChannel = "Microsoft-Windows-Windows Firewall With Advanced Security/Firewall"

func init() {
	fwProvider := "Microsoft-Windows-Windows Firewall With Advanced Security"
	fwGUID := "{d1bc9aff-2abf-4d71-9146-ecb2a986eb85}"

	fwEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{2004, LevelInformation, generateFWRuleAdded},
		{2005, LevelInformation, generateFWRuleModified},
		{2006, LevelInformation, generateFWRuleDeleted},
		{2009, LevelWarning, generateFWRuleApplyFailed},
		{2010, LevelWarning, generateFWProfileChange},
	}

	for _, ev := range fwEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      firewallChannel,
			Provider:     fwProvider,
			ProviderGUID: fwGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateFWRuleAdded(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	ruleNames := []string{"Allow-HTTP-In", "Allow-HTTPS-In", "Allow-RDP-In", "Block-Telnet-In", "Allow-SMB-In"}
	rule := ruleNames[r.Intn(len(ruleNames))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: rule},
		{Name: "Direction", Value: "1"},
		{Name: "Action", Value: "2"},
		{Name: "Profile", Value: "Public, Private, Domain"},
	}
	return data, fmt.Sprintf("A Windows Defender Firewall rule has been added.\n\nRule Name: %s", rule)
}

func generateFWRuleModified(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	rule := fmt.Sprintf("Custom-Rule-%d", r.Intn(100)) // #nosec G404
	data := []EventDataField{
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: rule},
		{Name: "Direction", Value: "1"},
		{Name: "Action", Value: "2"},
	}
	return data, fmt.Sprintf("A Windows Defender Firewall rule has been modified.\n\nRule Name: %s", rule)
}

func generateFWRuleDeleted(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	rule := fmt.Sprintf("Old-Rule-%d", r.Intn(100)) // #nosec G404
	data := []EventDataField{
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: rule},
	}
	return data, fmt.Sprintf("A Windows Defender Firewall rule has been deleted.\n\nRule Name: %s", rule)
}

func generateFWRuleApplyFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	rule := fmt.Sprintf("Problem-Rule-%d", r.Intn(100)) // #nosec G404
	data := []EventDataField{
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: rule},
		{Name: "ErrorCode", Value: "87"},
	}
	return data, fmt.Sprintf("Windows Defender Firewall could not apply the following rule:\n\nRule Name: %s\nError: 87 (parameter is incorrect)", rule)
}

func generateFWProfileChange(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	profiles := []string{"Domain", "Private", "Public"}
	profile := profiles[r.Intn(len(profiles))] // #nosec G404
	data := []EventDataField{
		{Name: "Profile", Value: profile},
		{Name: "State", Value: "ON"},
	}
	return data, fmt.Sprintf("A Windows Defender Firewall network profile has changed.\n\nProfile: %s\nState: ON", profile)
}
