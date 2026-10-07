package otlpgrpc

import (
	"fmt"
	"sort"
	"strings"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// entry is a prepared OTLP item (log record, metric or span) plus the
// resource of the generator that emitted it. The resource travels with the
// item through the channel and batch so a send can split the batch into one
// Resource{Logs,Metrics,Spans} per distinct resource.
type entry[T any] struct {
	item        T
	resource    map[string]any
	resourceKey string
}

func newEntry[T any](item T, resource map[string]any) *entry[T] {
	return &entry[T]{item: item, resource: resource, resourceKey: resourceKey(resource)}
}

// resourceGroup is the items of a batch that share one resource.
type resourceGroup[T any] struct {
	resource map[string]any
	items    []T
}

// groupByResource splits entries into one group per distinct resource, in
// order of first appearance. nil entries are skipped.
func groupByResource[T any](entries []*entry[T]) []*resourceGroup[T] {
	groups := make([]*resourceGroup[T], 0, 1)
	byKey := make(map[string]*resourceGroup[T])
	for _, e := range entries {
		if e == nil {
			continue
		}
		g, ok := byKey[e.resourceKey]
		if !ok {
			g = &resourceGroup[T]{resource: e.resource}
			byKey[e.resourceKey] = g
			groups = append(groups, g)
		}
		g.items = append(g.items, e.item)
	}
	return groups
}

// resourceAttributes returns the OTLP resource attributes for a generator
// resource: service.name=blitz unless the resource sets its own, followed by
// the resource's attributes in key order.
func resourceAttributes(resource map[string]any) []*commonpb.KeyValue {
	attrs := make([]*commonpb.KeyValue, 0, len(resource)+1)
	if _, ok := resource["service.name"]; !ok {
		attrs = append(attrs, &commonpb.KeyValue{
			Key:   "service.name",
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "blitz"}},
		})
	}
	for _, k := range sortedKeys(resource) {
		if av := toAnyValueSimple(resource[k]); av != nil {
			attrs = append(attrs, &commonpb.KeyValue{Key: k, Value: av})
		}
	}
	return attrs
}

// resourceKey returns a stable identity for a resource map, used to group
// items that share a resource.
func resourceKey(resource map[string]any) string {
	if len(resource) == 0 {
		return ""
	}
	var b strings.Builder
	for _, k := range sortedKeys(resource) {
		fmt.Fprintf(&b, "%q=%#v;", k, resource[k])
	}
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
