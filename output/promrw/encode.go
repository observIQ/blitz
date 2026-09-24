// Package promrw implements the prometheus-remote-write output: a push client
// that POSTs snappy-compressed remote-write payloads (1.0 or 2.0) to a
// configured endpoint. It serializes from the shared prommap representation.
package promrw

import (
	"fmt"
	"sort"

	"github.com/golang/snappy"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// version selects the remote-write wire protocol.
type version string

const (
	versionV1 version = "1.0"
	versionV2 version = "2.0"
)

// encode serializes the metric families into a snappy-compressed remote-write
// payload of the given version, ready to POST as the request body.
func encode(v version, families []prommap.MetricFamily) ([]byte, error) {
	switch v {
	case versionV1:
		return encodeV1(families)
	case versionV2:
		return encodeV2(families)
	default:
		return nil, fmt.Errorf("promrw: unsupported remote-write version %q", v)
	}
}

// sortedPairs returns a sample's labels as name/value pairs sorted by name,
// with __name__ prepended. Remote-write requires every series' labels sorted.
func sortedPairs(s prommap.Sample) [][2]string {
	pairs := make([][2]string, 0, len(s.Labels)+1)
	pairs = append(pairs, [2]string{"__name__", s.Name})
	for _, l := range s.Labels {
		pairs = append(pairs, [2]string{l.Name, l.Value})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	return pairs
}

func encodeV1(families []prommap.MetricFamily) ([]byte, error) {
	var req prompb.WriteRequest
	for _, fam := range families {
		for _, s := range fam.Samples {
			pairs := sortedPairs(s)
			labels := make([]prompb.Label, len(pairs))
			for i, p := range pairs {
				labels[i] = prompb.Label{Name: p[0], Value: p[1]}
			}
			req.Timeseries = append(req.Timeseries, prompb.TimeSeries{
				Labels:  labels,
				Samples: []prompb.Sample{{Value: s.Value, Timestamp: s.TimestampMS}},
			})
		}
	}
	raw, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("promrw: marshal v1 WriteRequest: %w", err)
	}
	return snappy.Encode(nil, raw), nil
}

func encodeV2(families []prommap.MetricFamily) ([]byte, error) {
	st := newSymbolTable()
	var req writev2.Request
	for _, fam := range families {
		for _, s := range fam.Samples {
			pairs := sortedPairs(s)
			refs := make([]uint32, 0, len(pairs)*2)
			for _, p := range pairs {
				refs = append(refs, st.intern(p[0]), st.intern(p[1]))
			}
			req.Timeseries = append(req.Timeseries, writev2.TimeSeries{
				LabelsRefs: refs,
				Samples:    []writev2.Sample{{Value: s.Value, Timestamp: s.TimestampMS}},
			})
		}
	}
	req.Symbols = st.symbols
	raw, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("promrw: marshal v2 Request: %w", err)
	}
	return snappy.Encode(nil, raw), nil
}

// symbolTable de-duplicates strings into the writev2 symbol array. Index 0 is
// always the empty string, as the v2 spec requires.
type symbolTable struct {
	symbols []string
	index   map[string]uint32
}

func newSymbolTable() *symbolTable {
	return &symbolTable{symbols: []string{""}, index: map[string]uint32{"": 0}}
}

func (t *symbolTable) intern(s string) uint32 {
	if ref, ok := t.index[s]; ok {
		return ref
	}
	ref := uint32(len(t.symbols)) // #nosec G115 -- symbol count is bounded by batch size, never near uint32 max
	t.symbols = append(t.symbols, s)
	t.index[s] = ref
	return ref
}
