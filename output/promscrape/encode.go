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
// emitted in name order, and label sets within a name in label order, for
// reproducible output. HELP and TYPE appear once per metric name: the registry
// holds one family per label set, and a repeated header is rejected by
// promtool and client_golang/expfmt consumers. When emitTimestamps is set,
// each sample line carries its millisecond timestamp; otherwise the scraper
// stamps at scrape time (the idiomatic exporter default). Metric and label
// names are assumed already sanitized by prommap.
func encode(families []prommap.MetricFamily, emitTimestamps bool) []byte {
	sorted := make([]prommap.MetricFamily, len(families))
	copy(sorted, families)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return seriesKey(sorted[i]) < seriesKey(sorted[j])
	})

	var b strings.Builder
	for i, fam := range sorted {
		if i == 0 || fam.Name != sorted[i-1].Name {
			writeHeader(&b, fam, sorted[i:])
		}
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

// writeHeader writes HELP (the first non-empty one for the name) and TYPE.
func writeHeader(b *strings.Builder, fam prommap.MetricFamily, rest []prommap.MetricFamily) {
	help := ""
	for _, f := range rest {
		if f.Name != fam.Name {
			break
		}
		if f.Help != "" {
			help = f.Help
			break
		}
	}
	if help != "" {
		b.WriteString("# HELP ")
		b.WriteString(fam.Name)
		b.WriteByte(' ')
		b.WriteString(helpEscaper.Replace(help))
		b.WriteByte('\n')
	}
	b.WriteString("# TYPE ")
	b.WriteString(fam.Name)
	b.WriteByte(' ')
	b.WriteString(string(fam.Type))
	b.WriteByte('\n')
}

// seriesKey orders label sets within a metric name.
func seriesKey(fam prommap.MetricFamily) string {
	if len(fam.Samples) == 0 {
		return ""
	}
	var b strings.Builder
	for _, l := range fam.Samples[0].Labels {
		b.WriteString(l.Name)
		b.WriteByte('\x00')
		b.WriteString(l.Value)
		b.WriteByte('\x00')
	}
	return b.String()
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
