package catalog

import (
	"fmt"
	"math/rand"
)

const terminalServicesChannel = "Microsoft-Windows-TerminalServices-LocalSessionManager/Operational"

func init() {
	tsProvider := "Microsoft-Windows-TerminalServices-LocalSessionManager"
	tsGUID := "{5d896912-022d-40aa-a3a8-4fa5515c76d7}"

	tsEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{21, LevelInformation, generateRDPSessionLogon},
		{23, LevelInformation, generateRDPSessionLogoff},
		{24, LevelInformation, generateRDPSessionDisconnect},
		{25, LevelInformation, generateRDPSessionReconnect},
		{39, LevelInformation, generateRDPSessionDisconnectByOther},
		{40, LevelInformation, generateRDPSessionDisconnectReason},
	}

	for _, ev := range tsEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      terminalServicesChannel,
			Provider:     tsProvider,
			ProviderGUID: tsGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateRDPSessionLogon(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	ip := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "SessionID", Value: sessionID},
		{Name: "Source", Value: ip},
	}
	return data, fmt.Sprintf("Remote Desktop Services: Session logon succeeded:\n\nUser: %s\\%s\nSession ID: %s\nSource Network Address: %s",
		opts.DomainName, user, sessionID, ip)
}

func generateRDPSessionLogoff(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	data := []EventDataField{
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "SessionID", Value: sessionID},
	}
	return data, fmt.Sprintf("Remote Desktop Services: Session logoff succeeded:\n\nUser: %s\\%s\nSession ID: %s",
		opts.DomainName, user, sessionID)
}

func generateRDPSessionDisconnect(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	ip := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "SessionID", Value: sessionID},
		{Name: "Source", Value: ip},
	}
	return data, fmt.Sprintf("Remote Desktop Services: Session has been disconnected:\n\nUser: %s\\%s\nSession ID: %s\nSource: %s",
		opts.DomainName, user, sessionID, ip)
}

func generateRDPSessionReconnect(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	ip := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "SessionID", Value: sessionID},
		{Name: "Source", Value: ip},
	}
	return data, fmt.Sprintf("Remote Desktop Services: Session reconnection succeeded:\n\nUser: %s\\%s\nSession ID: %s\nSource: %s",
		opts.DomainName, user, sessionID, ip)
}

func generateRDPSessionDisconnectByOther(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1)     // #nosec G404
	targetSession := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	data := []EventDataField{
		{Name: "SessionID", Value: sessionID},
		{Name: "TargetSessionID", Value: targetSession},
	}
	_ = opts
	return data, fmt.Sprintf("Session %s has been disconnected by session %s.", targetSession, sessionID)
}

func generateRDPSessionDisconnectReason(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	reasons := []string{"0", "5", "11", "12"}
	reason := reasons[r.Intn(len(reasons))]      // #nosec G404
	sessionID := fmt.Sprintf("%d", r.Intn(10)+1) // #nosec G404
	data := []EventDataField{
		{Name: "SessionID", Value: sessionID},
		{Name: "Reason", Value: reason},
	}
	_ = opts
	return data, fmt.Sprintf("Session %s has been disconnected, reason code %s.", sessionID, reason)
}
