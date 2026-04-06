package wel

import (
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/wel/catalog"
)

func TestEventRecordToXML(t *testing.T) {
	rec := &EventRecord{
		ProviderName:  "Microsoft-Windows-Security-Auditing",
		ProviderGUID:  "{54849625-5478-4994-a5ba-3e3b0328c30d}",
		EventID:       4624,
		Version:       2,
		Level:         catalog.LevelLogAlways,
		Task:          12544,
		TaskName:      "Logon",
		Opcode:        0,
		OpcodeName:    "Info",
		Keywords:      0x8020000000000000,
		KeywordNames:  []string{"Audit Success"},
		Channel:       "Security",
		Computer:      "WIN-SERVER01.contoso.com",
		TimeCreated:   time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
		EventRecordID: 12345,
		Data: []catalog.EventDataField{
			{Name: "SubjectUserSid", Value: "S-1-5-18"},
			{Name: "SubjectUserName", Value: "WIN-SERVER01$"},
			{Name: "SubjectDomainName", Value: "CONTOSO"},
			{Name: "TargetUserName", Value: "jsmith"},
			{Name: "LogonType", Value: "3"},
		},
		Message:   "An account was successfully logged on.",
		LevelName: "Information",
	}

	xml := rec.ToXML()

	// Verify XML structure
	checks := []string{
		`<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">`,
		`<Provider Name="Microsoft-Windows-Security-Auditing" Guid="{54849625-5478-4994-a5ba-3e3b0328c30d}"/>`,
		`<EventID>4624</EventID>`,
		`<Version>2</Version>`,
		`<Level>0</Level>`,
		`<Task>12544</Task>`,
		`<Opcode>0</Opcode>`,
		`<Keywords>0x8020000000000000</Keywords>`,
		`<TimeCreated SystemTime="2024-03-15T10:30:00.000000000Z"/>`,
		`<EventRecordID>12345</EventRecordID>`,
		`<Channel>Security</Channel>`,
		`<Computer>WIN-SERVER01.contoso.com</Computer>`,
		`<Data Name="SubjectUserSid">S-1-5-18</Data>`,
		`<Data Name="TargetUserName">jsmith</Data>`,
		`<Data Name="LogonType">3</Data>`,
		`<RenderingInfo>`,
		`<Message>An account was successfully logged on.</Message>`,
		`<Level>Information</Level>`,
		`<Task>Logon</Task>`,
		`<Opcode>Info</Opcode>`,
		`<Keyword>Audit Success</Keyword>`,
	}
	for _, check := range checks {
		if !strings.Contains(xml, check) {
			t.Errorf("XML missing expected content: %q\n\nFull XML:\n%s", check, xml)
		}
	}
}

func TestEventRecordToXMLNoGUID(t *testing.T) {
	rec := &EventRecord{
		ProviderName: "Service Control Manager",
		EventID:      7045,
		Level:        catalog.LevelInformation,
		Channel:      "System",
		Computer:     "WIN-SERVER01",
		TimeCreated:  time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
		Data: []catalog.EventDataField{
			{Name: "ServiceName", Value: "TestService"},
		},
	}

	xml := rec.ToXML()

	// Provider without GUID should not have Guid attribute
	if strings.Contains(xml, `Guid=""`) {
		t.Error("Provider without GUID should not have empty Guid attribute")
	}
	if !strings.Contains(xml, `<Provider Name="Service Control Manager"/>`) {
		t.Errorf("expected Provider without Guid, got:\n%s", xml)
	}
}

func TestEventRecordToXMLEmptyData(t *testing.T) {
	rec := &EventRecord{
		ProviderName: "TestProvider",
		EventID:      1,
		Channel:      "Application",
		Computer:     "TESTPC",
		TimeCreated:  time.Now(),
	}

	xml := rec.ToXML()

	// Should still produce valid XML even with no EventData
	if !strings.Contains(xml, "<EventData>") {
		t.Error("XML should contain EventData element even when empty")
	}
}

func TestEventRecordToXMLEscaping(t *testing.T) {
	rec := &EventRecord{
		ProviderName: "TestProvider",
		EventID:      1,
		Channel:      "Application",
		Computer:     "TESTPC",
		TimeCreated:  time.Now(),
		Data: []catalog.EventDataField{
			{Name: "Command", Value: `cmd /c "echo <test> & pause"`},
		},
		Message: "Command contains <special> & characters",
	}

	xml := rec.ToXML()

	if strings.Contains(xml, "<test>") {
		t.Error("XML should escape < in data values")
	}
	if !strings.Contains(xml, "&lt;test&gt;") {
		t.Error("XML should contain escaped < and > in data values")
	}
	if !strings.Contains(xml, "&amp; pause") {
		t.Error("XML should escape & in data values")
	}
}

func TestEventRecordToXMLEscapingSystemFields(t *testing.T) {
	rec := &EventRecord{
		ProviderName: "TestProvider",
		EventID:      1,
		Channel:      "App<&>Channel",
		Computer:     "host<&>name",
		TimeCreated:  time.Now(),
		TaskName:     "Task<&>Name",
		OpcodeName:   "Opcode<&>",
		KeywordNames: []string{"Audit<&>Success"},
		LevelName:    "Info<&>",
		Message:      "msg",
	}

	xml := rec.ToXML()

	for _, raw := range []string{
		"<Channel>App<&>Channel</Channel>",
		"<Computer>host<&>name</Computer>",
		"<Level>Info<&></Level>",
		"<Task>Task<&>Name</Task>",
		"<Opcode>Opcode<&></Opcode>",
		"<Keyword>Audit<&>Success</Keyword>",
	} {
		if strings.Contains(xml, raw) {
			t.Errorf("XML must not contain unescaped %q\n\nFull XML:\n%s", raw, xml)
		}
	}
	for _, escaped := range []string{
		"<Channel>App&lt;&amp;&gt;Channel</Channel>",
		"<Computer>host&lt;&amp;&gt;name</Computer>",
		"<Level>Info&lt;&amp;&gt;</Level>",
		"<Task>Task&lt;&amp;&gt;Name</Task>",
		"<Opcode>Opcode&lt;&amp;&gt;</Opcode>",
		"<Keyword>Audit&lt;&amp;&gt;Success</Keyword>",
	} {
		if !strings.Contains(xml, escaped) {
			t.Errorf("XML missing escaped form %q\n\nFull XML:\n%s", escaped, xml)
		}
	}
}

func TestStateTrackerLogonSessions(t *testing.T) {
	st := catalog.NewStateTracker(100)

	// Add some sessions
	st.AddLogonSession("0x1234", "jsmith", "CONTOSO")
	st.AddLogonSession("0x5678", "mjohnson", "CONTOSO")
	st.AddLogonSession("0xABCD", "bwilliams", "CONTOSO")

	t.Run("pick returns valid session", func(t *testing.T) {
		session, ok := st.PickLogonSession()
		if !ok {
			t.Fatal("expected to pick a session")
		}
		if session.LogonID == "" {
			t.Error("session LogonID should not be empty")
		}
		if session.Username == "" {
			t.Error("session Username should not be empty")
		}
	})

	t.Run("remove session", func(t *testing.T) {
		st2 := catalog.NewStateTracker(100)
		st2.AddLogonSession("0x1111", "alice", "DOMAIN")
		session, ok := st2.PickLogonSession()
		if !ok {
			t.Fatal("expected to pick a session")
		}
		st2.RemoveLogonSession(session.LogonID)
		_, ok = st2.PickLogonSession()
		if ok {
			t.Error("expected no session after removal")
		}
	})
}

func TestStateTrackerProcesses(t *testing.T) {
	st := catalog.NewStateTracker(100)

	st.AddProcess("0x100", `C:\Windows\System32\cmd.exe`, "jsmith")
	st.AddProcess("0x200", `C:\Windows\explorer.exe`, "mjohnson")

	t.Run("pick returns valid process", func(t *testing.T) {
		proc, ok := st.PickProcess()
		if !ok {
			t.Fatal("expected to pick a process")
		}
		if proc.ProcessID == "" {
			t.Error("process ID should not be empty")
		}
	})

	t.Run("remove process", func(t *testing.T) {
		st2 := catalog.NewStateTracker(100)
		st2.AddProcess("0x300", `C:\test.exe`, "user1")
		proc, ok := st2.PickProcess()
		if !ok {
			t.Fatal("expected to pick a process")
		}
		st2.RemoveProcess(proc.ProcessID)
		_, ok = st2.PickProcess()
		if ok {
			t.Error("expected no process after removal")
		}
	})
}

func TestStateTrackerBoundedMemory(t *testing.T) {
	maxEntries := 5
	st := catalog.NewStateTracker(maxEntries)

	// Add more entries than the max
	for i := 0; i < 10; i++ {
		st.AddLogonSession(strings.Repeat("A", i+1), "user", "DOMAIN")
	}

	// Count should be bounded
	if st.LogonSessionCount() > maxEntries {
		t.Errorf("expected at most %d logon sessions, got %d", maxEntries, st.LogonSessionCount())
	}
}

func TestStateTrackerEmpty(t *testing.T) {
	st := catalog.NewStateTracker(100)

	_, ok := st.PickLogonSession()
	if ok {
		t.Error("expected no session from empty tracker")
	}

	_, ok = st.PickProcess()
	if ok {
		t.Error("expected no process from empty tracker")
	}
}
