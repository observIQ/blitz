package hostmetrics

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessScraperName(t *testing.T) {
	s := &processScraper{}
	assert.Equal(t, "process", s.Name())
}

// Process identity must live in resource attributes, not datapoint attributes —
// that placement is what reduction pipelines key off.
func TestProcessScraperResourceAttributes(t *testing.T) {
	s := &processScraper{}
	r := rand.New(rand.NewSource(7)) // #nosec G404
	base := map[string]string{"host.name": "test", "os.type": "linux"}

	records := s.Scrape(r, "test-host", base)
	require.NotEmpty(t, records)

	for _, rec := range records {
		res := rec.Metadata.Resource
		assert.Equal(t, "test", res["host.name"], "base resource should be carried through")
		for _, key := range []string{
			"process.pid",
			"process.parent_pid",
			"process.executable.name",
			"process.executable.path",
			"process.command",
			"process.command_line",
			"process.owner",
		} {
			assert.NotEmpty(t, res[key], "%s: resource should carry %s", rec.Name, key)
		}
	}

	// The shared base map must not be mutated by the scrape.
	assert.Len(t, base, 2, "scrape must not write process attributes into the shared resource map")
}

// Every simulated process needs a distinct resource map; sharing one would
// collapse the cardinality the scraper exists to produce.
func TestProcessScraperDistinctResourcePerProcess(t *testing.T) {
	s := &processScraper{}
	r := rand.New(rand.NewSource(11)) // #nosec G404

	records := s.Scrape(r, "test-host", map[string]string{"host.name": "test", "os.type": "linux"})
	require.NotEmpty(t, records)

	executables := map[string]struct{}{}
	for _, rec := range records {
		executables[rec.Metadata.Resource["process.executable.name"]] = struct{}{}
	}
	assert.Len(t, executables, len(linuxProcesses), "each template should appear as its own process")
}

// The blueprint use case filters processes under 1 MiB resident, so a scrape has
// to contain some.
func TestProcessScraperEmitsSubMiBProcesses(t *testing.T) {
	s := &processScraper{}
	r := rand.New(rand.NewSource(3)) // #nosec G404

	records := s.Scrape(r, "test-host", map[string]string{"host.name": "test", "os.type": "linux"})

	var small, large int
	for _, rec := range records {
		if rec.Name != "process.memory.usage" {
			continue
		}
		require.NotNil(t, rec.IntValue)
		if *rec.IntValue < 1048576 {
			small++
		} else {
			large++
		}
	}
	assert.Positive(t, small, "expected at least one process under 1 MiB resident")
	assert.Positive(t, large, "expected at least one process over 1 MiB resident")
}

func TestProcessScraperWindows(t *testing.T) {
	s := &processScraper{}
	r := rand.New(rand.NewSource(19)) // #nosec G404

	records := s.Scrape(r, "test-host", map[string]string{"host.name": "test", "os.type": "windows"})
	require.NotEmpty(t, records)

	for _, rec := range records {
		res := rec.Metadata.Resource
		assert.True(t, strings.HasSuffix(res["process.executable.name"], ".exe"),
			"windows processes should use .exe names, got %q", res["process.executable.name"])
		assert.NotContains(t, res, "process.cgroup", "cgroup has no Windows equivalent")
	}
}

func TestProcessResourceCommandLine(t *testing.T) {
	t.Run("with args", func(t *testing.T) {
		tmpl := processTemplate{
			executable: "sshd", path: "/usr/sbin/sshd", cgroup: "/system.slice/ssh.service",
			owner: "root", args: []string{"-D", "-oPort=22"},
		}
		res := processResource(map[string]string{"host.name": "test"}, tmpl, 1842)
		assert.Equal(t, "/usr/sbin/sshd -D -oPort=22", res["process.command_line"])
		assert.Equal(t, "-D -oPort=22", res["process.command_args"])
		assert.Equal(t, "1842", res["process.pid"])
		assert.Equal(t, "/system.slice/ssh.service", res["process.cgroup"])
	})

	t.Run("without args", func(t *testing.T) {
		tmpl := processTemplate{executable: "cron", path: "/usr/sbin/cron", owner: "root"}
		res := processResource(map[string]string{}, tmpl, 42)
		assert.Equal(t, "/usr/sbin/cron", res["process.command_line"])
		assert.Empty(t, res["process.command_args"])
		assert.NotContains(t, res, "process.cgroup")
	})
}
