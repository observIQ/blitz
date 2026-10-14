package hostmetrics

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/observiq/blitz/output"
)

// processScraper produces per-process metrics equivalent to the OpenTelemetry
// hostmetrics receiver's `process` scraper. Unlike the system.* scrapers, each
// record carries its own resource map: the process identity (pid, executable,
// owner, cgroup, command line) lives in resource attributes, not datapoint
// attributes, which is what makes these metrics high cardinality and therefore
// the interesting case for reduction pipelines.
type processScraper struct{}

func (s *processScraper) Name() string { return "process" }

// processTemplate describes a single simulated process. Memory is expressed as
// a range so a scrape produces variation across cycles while keeping each
// process in a plausible band.
type processTemplate struct {
	executable  string
	path        string
	cgroup      string
	owner       string
	args        []string
	minMemoryKB int64
	maxMemoryKB int64
}

// Linux process table. systemd-journald and cron sit well under 1 MiB so a
// scrape always contains processes that memory-threshold filters can drop.
var linuxProcesses = []processTemplate{
	{"sshd", "/usr/sbin/sshd", "/system.slice/ssh.service", "root", []string{"-D", "-oCiphers=aes256-gcm@openssh.com"}, 512, 900},
	{"nginx", "/usr/sbin/nginx", "/system.slice/nginx.service", "www-data", []string{"-g", "daemon off;"}, 65536, 131072},
	{"postgres", "/usr/lib/postgresql/16/bin/postgres", "/system.slice/postgresql.service", "postgres", []string{"-D", "/var/lib/postgresql/16/main"}, 262144, 786432},
	{"dockerd", "/usr/bin/dockerd", "/system.slice/docker.service", "root", []string{"-H", "fd://", "--containerd=/run/containerd/containerd.sock"}, 131072, 262144},
	{"systemd-journald", "/lib/systemd/systemd-journald", "/system.slice/systemd-journald.service", "root", []string{}, 256, 800},
	{"cron", "/usr/sbin/cron", "/system.slice/cron.service", "root", []string{"-f"}, 128, 700},
	{"rsyslogd", "/usr/sbin/rsyslogd", "/system.slice/rsyslog.service", "syslog", []string{"-n", "-iNONE"}, 2048, 8192},
	{"prometheus-node-exporter", "/usr/bin/prometheus-node-exporter", "/system.slice/prometheus-node-exporter.service", "prometheus", []string{"--collector.systemd"}, 16384, 32768},
}

// Windows process table. Paths and owners follow Windows conventions; cgroup is
// left empty because it has no Windows equivalent.
var windowsProcesses = []processTemplate{
	{"sqlservr.exe", `C:\Program Files\Microsoft SQL Server\MSSQL16.MSSQLSERVER\MSSQL\Binn\sqlservr.exe`, "", `NT SERVICE\MSSQLSERVER`, []string{"-s", "MSSQLSERVER"}, 524288, 2097152},
	{"w3wp.exe", `C:\Windows\System32\inetsrv\w3wp.exe`, "", `IIS APPPOOL\DefaultAppPool`, []string{"-ap", "DefaultAppPool"}, 131072, 393216},
	{"MsMpEng.exe", `C:\Program Files\Windows Defender\MsMpEng.exe`, "", "LocalSystem", []string{}, 98304, 262144},
	{"spoolsv.exe", `C:\Windows\System32\spoolsv.exe`, "", "LocalSystem", []string{}, 512, 900},
	{"svchost.exe", `C:\Windows\System32\svchost.exe`, "", `NT AUTHORITY\NETWORK SERVICE`, []string{"-k", "netsvcs", "-p"}, 8192, 40960},
	{"wininit.exe", `C:\Windows\System32\wininit.exe`, "", "LocalSystem", []string{}, 256, 800},
}

func (s *processScraper) Scrape(r *rand.Rand, _ string, resource map[string]string) []output.MetricRecord {
	now := time.Now()

	templates := linuxProcesses
	if resource["os.type"] == "windows" {
		templates = windowsProcesses
	}

	var records []output.MetricRecord
	for _, tmpl := range templates {
		pid := int64(r.Intn(30000) + 100) // #nosec G404
		res := processResource(resource, tmpl, pid)

		memKB := tmpl.minMemoryKB
		if span := tmpl.maxMemoryKB - tmpl.minMemoryKB; span > 0 {
			memKB += r.Int63n(span) // #nosec G404
		}
		rss := memKB * 1024
		// Virtual memory runs 2-4x resident for a typical daemon.
		virtual := rss * int64(2+r.Intn(3)) // #nosec G404

		records = append(records,
			output.MetricRecord{
				Name: "process.memory.usage", Description: "The amount of physical memory in use",
				Unit: "By", Type: output.MetricTypeGauge,
				IntValue: int64Ptr(rss),
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{},
					Resource:   res,
				},
			},
			output.MetricRecord{
				Name: "process.memory.virtual", Description: "Virtual memory size",
				Unit: "By", Type: output.MetricTypeGauge,
				IntValue: int64Ptr(virtual),
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{},
					Resource:   res,
				},
			},
			output.MetricRecord{
				Name: "process.threads", Description: "Process threads count",
				Unit: "{thread}", Type: output.MetricTypeGauge,
				IntValue: int64Ptr(int64(r.Intn(64) + 1)), // #nosec G404
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{},
					Resource:   res,
				},
			},
			output.MetricRecord{
				Name: "process.open_file_descriptors", Description: "Number of file descriptors in use by the process",
				Unit: "{count}", Type: output.MetricTypeGauge,
				IntValue: int64Ptr(int64(r.Intn(512) + 3)), // #nosec G404
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{},
					Resource:   res,
				},
			},
		)

		for _, state := range []string{"user", "system"} {
			records = append(records, output.MetricRecord{
				Name: "process.cpu.time", Description: "Total CPU seconds broken down by different states",
				Unit: "s", Type: output.MetricTypeSum,
				DoubleValue: float64Ptr(r.Float64() * 1000), // #nosec G404
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{"state": state},
					Resource:   res,
				},
			})
		}

		for _, direction := range []string{"read", "write"} {
			records = append(records, output.MetricRecord{
				Name: "process.disk.io", Description: "Disk bytes transferred",
				Unit: "By", Type: output.MetricTypeSum,
				IntValue: int64Ptr(int64(r.Intn(1 << 30))), // #nosec G404
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: map[string]string{"direction": direction},
					Resource:   res,
				},
			})
		}
	}

	return records
}

// processResource copies the shared host resource and layers the per-process
// identity attributes on top, so every process gets its own resource map and
// no scrape mutates the caller's map. process.cgroup is omitted when the
// template has none (Windows).
func processResource(base map[string]string, tmpl processTemplate, pid int64) map[string]string {
	res := make(map[string]string, len(base)+8)
	for k, v := range base {
		res[k] = v
	}

	commandLine := tmpl.path
	if len(tmpl.args) > 0 {
		commandLine = tmpl.path + " " + strings.Join(tmpl.args, " ")
	}

	res["process.pid"] = fmt.Sprintf("%d", pid)
	res["process.parent_pid"] = "1"
	res["process.executable.name"] = tmpl.executable
	res["process.executable.path"] = tmpl.path
	res["process.command"] = tmpl.path
	res["process.command_line"] = commandLine
	res["process.command_args"] = strings.Join(tmpl.args, " ")
	res["process.owner"] = tmpl.owner
	if tmpl.cgroup != "" {
		res["process.cgroup"] = tmpl.cgroup
	}

	return res
}
