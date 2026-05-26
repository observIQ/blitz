package catalog

import (
	"fmt"
	"math/rand"
)

const defenderChannel = "Microsoft-Windows-Windows Defender/Operational"

func init() {
	defProvider := "Microsoft-Windows-Windows Defender"
	defGUID := "{11cd958a-c507-4ef3-b3f2-5fd9dfbd2c78}"

	defenderEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{1000, LevelInformation, generateDefenderScanStarted},
		{1001, LevelInformation, generateDefenderScanCompleted},
		{1002, LevelInformation, generateDefenderScanCancelled},
		{1006, LevelWarning, generateDefenderMalwareDetected},
		{1007, LevelInformation, generateDefenderMalwareAction},
		{1116, LevelWarning, generateDefenderThreatDetected},
		{1117, LevelInformation, generateDefenderThreatAction},
		{2000, LevelInformation, generateDefenderSignatureUpdated},
		{2001, LevelError, generateDefenderSignatureUpdateFailed},
		{5001, LevelInformation, generateDefenderRTPDisabled},
		{5007, LevelInformation, generateDefenderConfigChanged},
	}

	for _, ev := range defenderEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      defenderChannel,
			Provider:     defProvider,
			ProviderGUID: defGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateDefenderScanStarted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	scanTypes := []string{"AntiVirus", "AntiSpyware"}
	scanType := scanTypes[r.Intn(len(scanTypes))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "Scan ID", Value: RandomGUID(r)},
		{Name: "Scan Type", Value: scanType},
		{Name: "Scan Parameters", Value: "Quick Scan"},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Windows Defender %s scan has started.\nScan Type: Quick Scan\nUser: %s\\%s", scanType, opts.DomainName, user)
}

func generateDefenderScanCompleted(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Scan ID", Value: RandomGUID(r)},
		{Name: "Scan Type", Value: "AntiVirus"},
		{Name: "Scan Parameters", Value: "Quick Scan"},
		{Name: "Scan Time", Value: "300"},
	}
	return data, "Windows Defender scan has completed.\nScan Type: Quick Scan\nScan Time: 300 seconds"
}

func generateDefenderScanCancelled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "Scan ID", Value: RandomGUID(r)},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Windows Defender scan has been stopped before completion.\nUser: %s\\%s", opts.DomainName, user)
}

func generateDefenderMalwareDetected(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	threats := []string{"Trojan:Win32/Generic", "PUA:Win32/Presenoker", "HackTool:Win32/Mimikatz", "Backdoor:Win32/Cobalt"}
	threat := threats[r.Intn(len(threats))] // #nosec G404
	data := []EventDataField{
		{Name: "Threat Name", Value: threat},
		{Name: "Severity", Value: "High"},
		{Name: "Category", Value: "Malware"},
		{Name: "Path", Value: `file:_C:\Users\Public\Downloads\suspicious.exe`},
		{Name: "Detection Source", Value: "Real-Time Protection"},
	}
	return data, fmt.Sprintf("Windows Defender has detected malware or other potentially unwanted software.\nName: %s\nSeverity: High", threat)
}

func generateDefenderMalwareAction(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	actions := []string{"Quarantine", "Remove", "Allow", "Block"}
	action := actions[r.Intn(len(actions))] // #nosec G404
	data := []EventDataField{
		{Name: "Threat Name", Value: "Trojan:Win32/Generic"},
		{Name: "Action", Value: action},
		{Name: "Status", Value: "Success"},
	}
	return data, fmt.Sprintf("Windows Defender has taken action: %s\nThreat: Trojan:Win32/Generic\nStatus: Success", action)
}

func generateDefenderThreatDetected(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	threats := []string{"PUA:Win32/Presenoker", "Adware:Win32/BrowseFox", "TrojanDropper:Win32/Sality"}
	threat := threats[r.Intn(len(threats))] // #nosec G404
	data := []EventDataField{
		{Name: "Threat Name", Value: threat},
		{Name: "Severity", Value: "Moderate"},
		{Name: "Detection Source", Value: "Downloads and attachments"},
		{Name: "Process Name", Value: `C:\Windows\explorer.exe`},
	}
	return data, fmt.Sprintf("Windows Defender Antivirus has detected a threat.\nThreat: %s\nSeverity: Moderate", threat)
}

func generateDefenderThreatAction(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Threat Name", Value: "PUA:Win32/Presenoker"},
		{Name: "Action", Value: "Quarantine"},
		{Name: "Error Code", Value: "0x0"},
		{Name: "Error Description", Value: "The operation completed successfully."},
	}
	_ = r
	return data, "Windows Defender Antivirus has taken action against a threat.\nThreat: PUA:Win32/Presenoker\nAction: Quarantine"
}

func generateDefenderSignatureUpdated(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	ver := fmt.Sprintf("1.%d.%d.0", 400+r.Intn(100), r.Intn(10)) // #nosec G404
	data := []EventDataField{
		{Name: "New Signature Version", Value: ver},
		{Name: "Previous Signature Version", Value: fmt.Sprintf("1.%d.%d.0", 399+r.Intn(100), r.Intn(10))}, // #nosec G404
		{Name: "Signature Type", Value: "AntiVirus"},
		{Name: "Update Type", Value: "Full"},
		{Name: "Update Source", Value: "Microsoft Update Server"},
	}
	return data, fmt.Sprintf("Windows Defender Antivirus definitions updated successfully.\nNew Version: %s", ver)
}

func generateDefenderSignatureUpdateFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Error Code", Value: "0x80072ee7"},
		{Name: "Error Description", Value: "The server name or address could not be resolved"},
		{Name: "Update Source", Value: "Microsoft Update Server"},
	}
	_ = r
	return data, "Windows Defender Antivirus definitions update failed.\nError: 0x80072ee7 - The server name or address could not be resolved"
}

func generateDefenderRTPDisabled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "Feature", Value: "Real-Time Protection"},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Real-Time Protection was disabled.\nUser: %s\\%s", opts.DomainName, user)
}

func generateDefenderConfigChanged(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Old Value", Value: "0x0"},
		{Name: "New Value", Value: "0x1"},
		{Name: "Feature", Value: "Cloud-delivered protection"},
	}
	_ = r
	return data, "Windows Defender Antivirus configuration has changed.\nFeature: Cloud-delivered protection\nOld value: 0x0\nNew value: 0x1"
}
