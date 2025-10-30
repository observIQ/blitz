package templates

import (
	"fmt"
	"math/rand"
	"strings"
)

// RenderOptions controls rendering of a Windows Event template.
type RenderOptions struct {
	// TemplateName selects which built-in template to use.
	TemplateName string
	// IPs is a list of candidate IP addresses to choose from.
	IPs []string
}

// DefaultIPs provides fallback IP candidates if none are configured.
var DefaultIPs = []string{
	"103.165.114.4",
	"192.0.2.10",
	"198.51.100.23",
	"203.0.113.77",
	"10.0.0.5",
}

// RenderTemplate renders the selected template with randomized values.
func RenderTemplate(opts RenderOptions) ([]byte, error) {
	templateName := opts.TemplateName
	if templateName == "" {
		templateName = ExampleTemplateName
	}

	templates := AllTemplates()
	tpl, ok := templates[templateName]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", opts.TemplateName)
	}

	ips := opts.IPs
	if len(ips) == 0 {
		ips = DefaultIPs
	}

	ip := ips[rand.Intn(len(ips))] // #nosec G404 - non-crypto random is fine

	out := strings.ReplaceAll(tpl, "{{IP_ADDRESS}}", ip)
	return []byte(out), nil
}
