package runtime_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/observiq/blitz/internal/runtime"
	"go.uber.org/zap/zaptest"
)

type recordingModule struct {
	name      string
	startCnt  atomic.Int32
	stopCnt   atomic.Int32
	startErr  error
	stopErr   error
	startCall *atomic.Int32 // shared counter to record call order
	startSeq  int32
	stopCall  *atomic.Int32
	stopSeq   int32
}

func (m *recordingModule) Name() string { return m.name }

func (m *recordingModule) Start(_ context.Context) error {
	m.startCnt.Add(1)
	if m.startCall != nil {
		m.startSeq = m.startCall.Add(1)
	}
	return m.startErr
}

func (m *recordingModule) Stop(_ context.Context) error {
	m.stopCnt.Add(1)
	if m.stopCall != nil {
		m.stopSeq = m.stopCall.Add(1)
	}
	return m.stopErr
}

func TestRuntime_StartCallsEveryModuleInOrder(t *testing.T) {
	startOrder := &atomic.Int32{}
	a := &recordingModule{name: "a", startCall: startOrder}
	b := &recordingModule{name: "b", startCall: startOrder}
	c := &recordingModule{name: "c", startCall: startOrder}

	rt := runtime.New(zaptest.NewLogger(t), []runtime.Module{a, b, c})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if a.startCnt.Load() != 1 || b.startCnt.Load() != 1 || c.startCnt.Load() != 1 {
		t.Fatalf("expected each module started once, got a=%d b=%d c=%d",
			a.startCnt.Load(), b.startCnt.Load(), c.startCnt.Load())
	}
	if a.startSeq != 1 || b.startSeq != 2 || c.startSeq != 3 {
		t.Fatalf("expected start order a<b<c, got a=%d b=%d c=%d",
			a.startSeq, b.startSeq, c.startSeq)
	}
}

func TestRuntime_StartRollsBackOnFailure(t *testing.T) {
	stopOrder := &atomic.Int32{}
	a := &recordingModule{name: "a", stopCall: stopOrder}
	b := &recordingModule{name: "b", stopCall: stopOrder}
	failing := &recordingModule{name: "failing", startErr: errors.New("boom")}

	rt := runtime.New(zaptest.NewLogger(t), []runtime.Module{a, b, failing})
	err := rt.Start(context.Background())
	if err == nil {
		t.Fatal("expected error from Start")
	}

	// a and b should have been stopped during rollback; failing was never started.
	if a.stopCnt.Load() != 1 {
		t.Errorf("expected a stopped once, got %d", a.stopCnt.Load())
	}
	if b.stopCnt.Load() != 1 {
		t.Errorf("expected b stopped once, got %d", b.stopCnt.Load())
	}
	if failing.stopCnt.Load() != 0 {
		t.Errorf("expected failing module not stopped, got %d", failing.stopCnt.Load())
	}
	// Rollback should stop in reverse: b before a.
	if b.stopSeq != 1 || a.stopSeq != 2 {
		t.Errorf("expected rollback reverse order b(1),a(2), got a=%d b=%d", a.stopSeq, b.stopSeq)
	}
}

func TestRuntime_StopCallsEveryModuleInReverseOrder(t *testing.T) {
	stopOrder := &atomic.Int32{}
	a := &recordingModule{name: "a", stopCall: stopOrder}
	b := &recordingModule{name: "b", stopCall: stopOrder}
	c := &recordingModule{name: "c", stopCall: stopOrder}

	rt := runtime.New(zaptest.NewLogger(t), []runtime.Module{a, b, c})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if c.stopSeq != 1 || b.stopSeq != 2 || a.stopSeq != 3 {
		t.Fatalf("expected stop reverse order c<b<a, got a=%d b=%d c=%d",
			a.stopSeq, b.stopSeq, c.stopSeq)
	}
}

func TestRuntime_StopContinuesOnError(t *testing.T) {
	a := &recordingModule{name: "a"}
	b := &recordingModule{name: "b", stopErr: errors.New("b-stop-fail")}
	c := &recordingModule{name: "c"}

	rt := runtime.New(zaptest.NewLogger(t), []runtime.Module{a, b, c})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := rt.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error from Stop")
	}
	// Even though b failed, a and c should still have been stopped.
	if a.stopCnt.Load() != 1 || c.stopCnt.Load() != 1 {
		t.Errorf("expected a and c stopped once each, got a=%d c=%d",
			a.stopCnt.Load(), c.stopCnt.Load())
	}
}

func TestRuntime_NewWithNilLoggerUsesNop(t *testing.T) {
	rt := runtime.New(nil, nil)
	// Should not panic with empty modules.
	if err := rt.Start(context.Background()); err != nil {
		t.Errorf("Start with empty modules and nil logger: %v", err)
	}
	if err := rt.Stop(context.Background()); err != nil {
		t.Errorf("Stop with empty modules and nil logger: %v", err)
	}
}
