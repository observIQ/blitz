package templates

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// RenderOptions controls rendering of a Windows Event template.
type RenderOptions struct {
	// TemplateName selects which built-in template to use.
	// If empty, a random template will be selected.
	TemplateName string
	// IPs is a list of candidate IP addresses to choose from.
	IPs []string
	// Hostnames is a list of candidate hostnames to choose from.
	Hostnames []string
}

// DefaultIPs provides fallback IP candidates if none are configured.
var DefaultIPs = []string{
	"103.165.114.4",
	"192.0.2.10",
	"198.51.100.23",
	"203.0.113.77",
	"10.0.0.5",
}

// DefaultHostnames provides fallback hostname candidates if none are configured.
var DefaultHostnames = []string{
	"iis-east1-prd-0",
	"web-west2-stg-1",
	"db-north1-prod-2",
	"app-south1-dev-3",
	"cache-central1-prod-4",
	"api-east2-stg-5",
	"worker-west1-prod-6",
	"monitor-north2-prod-7",
	"gateway-south2-stg-8",
	"loadbalancer-central2-prod-9",
}

// templateNames is a pre-computed list of all available template names for efficient random selection.
var templateNames = []string{
	ExampleTemplateName,
	ServiceControlManagerTemplateName,
	SuccessfulLogonTemplateName,
}

// AllTemplates returns a map of built-in Windows Event templates.
func AllTemplates() map[string]string {
	return map[string]string{
		ExampleTemplateName:               exampleXMLTemplate,
		ServiceControlManagerTemplateName: serviceControlManagerXMLTemplate,
		SuccessfulLogonTemplateName:       successfulLogonXMLTemplate,
	}
}

// RenderTemplate renders the selected template with randomized values.
// If TemplateName is empty, a random template is selected.
func RenderTemplate(opts RenderOptions) (string, error) {
	// Use seeded random source for better randomness
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 - non-crypto random is fine

	templates := AllTemplates()

	templateName := opts.TemplateName
	if templateName == "" {
		// Randomly select a template name
		templateName = templateNames[r.Intn(len(templateNames))] // #nosec G404 - non-crypto random is fine
	}

	tpl, ok := templates[templateName]
	if !ok {
		return "", fmt.Errorf("unknown template: %s", templateName)
	}

	ips := opts.IPs
	if len(ips) == 0 {
		ips = DefaultIPs
	}

	ip := ips[r.Intn(len(ips))] // #nosec G404 - non-crypto random is fine

	hostnames := opts.Hostnames
	if len(hostnames) == 0 {
		hostnames = DefaultHostnames
	}

	hostname := hostnames[r.Intn(len(hostnames))] // #nosec G404 - non-crypto random is fine
	hostnameUpper := strings.ToUpper(hostname)
	hostnameLower := strings.ToLower(hostname)

	out := strings.ReplaceAll(tpl, "{{IP_ADDRESS}}", ip)
	out = strings.ReplaceAll(out, "{{HOSTNAME_UPPER}}", hostnameUpper)
	out = strings.ReplaceAll(out, "{{HOSTNAME_LOWER}}", hostnameLower)
	return out, nil
}
