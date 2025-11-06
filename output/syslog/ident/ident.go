package ident

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	seedCounter uint64
	randPool    = sync.Pool{
		New: func() any {
			// Unique-ish seeds for each pool instance; not crypto-secure.
			s := time.Now().UnixNano() + int64(atomic.AddUint64(&seedCounter, 1))
			return rand.New(rand.NewSource(s)) // #nosec G404
		},
	}
)

// RandomAppName returns a random app name.
func RandomAppName() string {
	return fromList(appNames)
}

// RandomHostname returns a random hostname.
func RandomHostname() string {
	return fromList(hostnames)
}

// RandomProcID returns a random process identifier.
func RandomProcID() string {
	return fromList(procIDs)
}

// RandomMsgID returns a random message identifier.
func RandomMsgID() string {
	return fromList(msgIDs)
}

func fromList(list []string) string {
	if len(list) == 0 {
		return "blitz"
	}
	r := randPool.Get().(*rand.Rand)
	idx := r.Intn(len(list))
	randPool.Put(r)
	return list[idx]
}

var appNames = []string{
	"blitz",
	"sysmon",
	"collector",
	"ingestor",
	"forwarder",
	"logger",
	"agent",
	"pipeline",
	"processor",
}

var hostnames = []string{
	"host01",
	"host02",
	"edge-1",
	"edge-2",
	"srv-01",
	"srv-02",
	"gateway",
	"core-1",
	"node-a",
	"node-b",
}

var procIDs = []string{
	"123",
	"456",
	"789",
	"pid42",
	"1",
	"100",
	"200",
	"999",
}

var msgIDs = []string{
	"startup",
	"config",
	"reload",
	"heartbeat",
	"event",
	"traffic",
	"threat",
	"status",
}
