package promscrape

import (
	"sort"
	"strconv"
	"strings"

	"github.com/observiq/blitz/internal/prommap"
)

// helpEscaper escapes a HELP line: backslash and newline only.
var helpEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`)

// labelEscaper escapes a label value: backslash, newline, and double-quote.
var labelEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)

// encode renders families to Prometheus text exposition format. Families are
// emitted in name order for reproducible output. When emitTimestamps is set,
// each sample line carries its millisecond timestamp; otherwise the scraper
// stamps at scrape time (the idiomatic exporter default). Metric and label
// names are assumed already sanitized by prommap.
func encode(families []prommap.MetricFamily, emitTimestamps bool) []byte {
	sorted := make([]prommap.MetricFamily, len(families))
	copy(sorted, families)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var b strings.Builder
	for _, fam := range sorted {
		if fam.Help != "" {
			b.WriteString("# HELP ")
			b.WriteString(fam.Name)
			b.WriteByte(' ')
			b.WriteString(helpEscaper.Replace(fam.Help))
			b.WriteByte('\n')
		}
		b.WriteString("# TYPE ")
		b.WriteString(fam.Name)
		b.WriteByte(' ')
		b.WriteString(string(fam.Type))
		b.WriteByte('\n')

		for _, s := range fam.Samples {
			b.WriteString(s.Name)
			writeLabels(&b, s.Labels)
			b.WriteByte(' ')
			b.WriteString(strconv.FormatFloat(s.Value, 'g', -1, 64))
			if emitTimestamps {
				b.WriteByte(' ')
				b.WriteString(strconv.FormatInt(s.TimestampMS, 10))
			}
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

// writeLabels writes {name="value",...}, or nothing when there are no labels.
func writeLabels(b *strings.Builder, labels []prommap.Label) {
	if len(labels) == 0 {
		return
	}
	b.WriteByte('{')
	for i, l := range labels {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(l.Name)
		b.WriteString(`="`)
		b.WriteString(labelEscaper.Replace(l.Value))
		b.WriteByte('"')
	}
	b.WriteByte('}')
}
