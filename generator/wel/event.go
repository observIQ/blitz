package wel

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/wel/catalog"
)

// EventRecord is a fully populated Windows Event ready for writing and XML rendering.
type EventRecord struct {
	// System fields
	ProviderName string
	ProviderGUID string
	EventID      int
	Version      int
	Level        catalog.EventLevel
	Task         int
	TaskName     string
	Opcode       int
	OpcodeName   string
	Keywords     uint64
	KeywordNames []string
	Channel      string
	Computer     string

	// Auto-populated at generation time
	TimeCreated   time.Time
	EventRecordID int64

	// EventData — ordered key-value pairs
	Data []catalog.EventDataField

	// RenderingInfo
	Message   string
	LevelName string
}

// ToXML renders the event as standard WEL XML matching the
// http://schemas.microsoft.com/win/2004/08/events/event schema.
func (e *EventRecord) ToXML() string {
	var b strings.Builder

	b.WriteString(`<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">`)
	b.WriteByte('\n')

	// System
	b.WriteString("  <System>\n")

	// Provider
	if e.ProviderGUID != "" {
		fmt.Fprintf(&b, "    <Provider Name=%q Guid=%q/>\n", e.ProviderName, e.ProviderGUID)
	} else {
		fmt.Fprintf(&b, "    <Provider Name=%q/>\n", e.ProviderName)
	}

	fmt.Fprintf(&b, "    <EventID>%d</EventID>\n", e.EventID)
	fmt.Fprintf(&b, "    <Version>%d</Version>\n", e.Version)
	fmt.Fprintf(&b, "    <Level>%d</Level>\n", e.Level)
	fmt.Fprintf(&b, "    <Task>%d</Task>\n", e.Task)
	fmt.Fprintf(&b, "    <Opcode>%d</Opcode>\n", e.Opcode)
	fmt.Fprintf(&b, "    <Keywords>%s</Keywords>\n", catalog.KeywordsToHex(e.Keywords))
	fmt.Fprintf(&b, "    <TimeCreated SystemTime=%q/>\n", e.TimeCreated.Format("2006-01-02T15:04:05.000000000Z"))
	fmt.Fprintf(&b, "    <EventRecordID>%d</EventRecordID>\n", e.EventRecordID)
	fmt.Fprintf(&b, "    <Channel>%s</Channel>\n", xmlEscape(e.Channel))
	fmt.Fprintf(&b, "    <Computer>%s</Computer>\n", xmlEscape(e.Computer))

	b.WriteString("  </System>\n")

	// EventData
	b.WriteString("  <EventData>\n")
	for _, d := range e.Data {
		fmt.Fprintf(&b, "    <Data Name=%q>%s</Data>\n", d.Name, xmlEscape(d.Value))
	}
	b.WriteString("  </EventData>\n")

	// RenderingInfo
	if e.Message != "" || e.LevelName != "" || e.TaskName != "" {
		b.WriteString("  <RenderingInfo Culture=\"en-US\">\n")
		if e.Message != "" {
			fmt.Fprintf(&b, "    <Message>%s</Message>\n", xmlEscape(e.Message))
		}
		if e.LevelName != "" {
			fmt.Fprintf(&b, "    <Level>%s</Level>\n", xmlEscape(e.LevelName))
		}
		if e.TaskName != "" {
			fmt.Fprintf(&b, "    <Task>%s</Task>\n", xmlEscape(e.TaskName))
		}
		if e.OpcodeName != "" {
			fmt.Fprintf(&b, "    <Opcode>%s</Opcode>\n", xmlEscape(e.OpcodeName))
		}
		if len(e.KeywordNames) > 0 {
			b.WriteString("    <Keywords>\n")
			for _, kw := range e.KeywordNames {
				fmt.Fprintf(&b, "      <Keyword>%s</Keyword>\n", xmlEscape(kw))
			}
			b.WriteString("    </Keywords>\n")
		}
		b.WriteString("  </RenderingInfo>\n")
	}

	b.WriteString("</Event>")

	return b.String()
}

// xmlEscape performs minimal XML escaping for attribute/text content.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// GenerateRecord creates an EventRecord from an EventDefinition and options.
func GenerateRecord(rng *rand.Rand, def *catalog.EventDefinition, opts *catalog.GenerateOpts, recordID int64) *EventRecord {
	data, message := def.Generate(rng, opts)

	levelName := def.Level.String()
	if def.Level == catalog.LevelLogAlways {
		// Security audit events use Level 0 but display as "Information"
		levelName = "Information"
	}

	return &EventRecord{
		ProviderName:  def.Provider,
		ProviderGUID:  def.ProviderGUID,
		EventID:       def.EventID,
		Version:       def.Version,
		Level:         def.Level,
		Task:          def.Task,
		TaskName:      def.TaskName,
		Opcode:        def.Opcode,
		OpcodeName:    def.OpcodeName,
		Keywords:      def.Keywords,
		KeywordNames:  def.KeywordNames,
		Channel:       def.Channel,
		Computer:      opts.Computer,
		TimeCreated:   time.Now(),
		EventRecordID: recordID,
		Data:          data,
		Message:       message,
		LevelName:     levelName,
	}
}
