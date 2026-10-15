package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunGivesSameTypeEntriesTheirOwnHosts pins the CLI's per-entry ordinals:
// two generators: entries of one type each describe their own host.
func TestRunGivesSameTypeEntriesTheirOwnHosts(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.log")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	yaml := fmt.Sprintf(`generators:
  - type: json
    count: 400
    json:
      workers: 1
      rate: 1ms
  - type: json
    count: 400
    json:
      workers: 1
      rate: 1ms
output:
  type: file
  file:
    path: %s
metrics:
  port: %d
`, outPath, port)
	path := filepath.Join(dir, "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	require.NoError(t, run(nil, nil))

	f, err := os.Open(outPath) // #nosec G304 -- test temp file
	require.NoError(t, err)
	defer f.Close()
	hosts := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec struct {
			Host string `json:"host"`
		}
		require.NoError(t, json.Unmarshal(sc.Bytes(), &rec), "line %q", sc.Text())
		hosts[rec.Host] = true
	}
	require.NoError(t, sc.Err())
	require.Len(t, hosts, 2, "each json entry describes its own host: %v", hosts)
}
