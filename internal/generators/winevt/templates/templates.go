package templates

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/observiq/blitz/internal/datagen"
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

// DefaultIPs provides fallback IP candidates generated from a fixed seed.
var DefaultIPs = func() []string {
	r := rand.New(rand.NewSource(42)) // #nosec G404 - deterministic default
	ips := make([]string, 5)
	for i := range ips {
		ips[i] = datagen.RandomPrivateIPv4(r)
	}
	return ips
}()

// DefaultHostnames provides fallback hostname candidates using mythology names.
var DefaultHostnames = datagen.GenerateHostnames(42, 10, datagen.StyleWindows, datagen.RomanNames)

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
