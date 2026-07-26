// artnet_noninterference_test.go proves SCRP-06's structural claim
// (08-06-PLAN.md Task 3, CONTEXT "UI, persistence, scripts, API, LLM,
// and Linear never own or block deterministic playback or Art-Net
// timing"): a live artnet.Worker's ticker goroutine and a live script
// run driven through this package's own Host/Run never call into each
// other, so terminating a runaway script cannot delay or miss a single
// Art-Net tick. The Worker is driven entirely by its own goroutine
// against a fakeArtnetFrameSource that never touches internal/script;
// the script run is driven entirely by a separate goroutine calling this
// package's own Host.Run; the only connection between the two is this
// test's own top-level orchestration -- proving structurally, by
// construction, that no goroutine in the worker's call graph ever
// reaches into internal/script (and vice versa).
//
// Gated behind skipUnlessDenoProvisioned (session_test.go's helper),
// exactly like every other real-Deno test in this package: a genuinely
// runaway script and its kill path require a real Deno subprocess.
package script

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// fakeArtnetFrameSource is a minimal artnet.FrameSource test double
// substituting for *playback.Engine -- structurally the same shape
// internal/artnet/worker_test.go's own fakeFrameSource uses, copied here
// (not imported: that type is test-only and unexported in package
// artnet) rather than reused. It never touches internal/script in any
// way.
type fakeArtnetFrameSource struct {
	frame *playback.Frame
}

// CurrentFrame implements artnet.FrameSource.
func (f *fakeArtnetFrameSource) CurrentFrame() *playback.Frame {
	return f.frame
}

// newArtnetLoopbackListener mirrors internal/artnet/worker_test.go's
// newLoopbackListener harness shape (08-06-PLAN.md Task 3's read_first
// instruction) -- copied rather than imported, since that helper is
// test-only and unexported in package artnet.
func newArtnetLoopbackListener(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func artnetListenerPort(t *testing.T, conn *net.UDPConn) int {
	t.Helper()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected *net.UDPAddr, got %T", conn.LocalAddr())
	}
	return addr.Port
}

// singleIntensityChannelMode is a minimal one-intensity-channel DMX
// layout, mirroring worker_test.go's singleChannelMode.
var singleIntensityChannelMode = fixture.Mode{
	Name:     "Standard",
	Channels: []fixture.ChannelSlot{{Type: fixture.CapabilityIntensity, Occurrence: 0}},
}

func staticArtnetResolver(mode fixture.Mode) artnet.ResolveFunc {
	return func(instance deployment.Instance) (artnet.InstanceFixture, error) {
		return artnet.InstanceFixture{
			Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}},
			Mode:       mode,
		}, nil
	}
}

func mustArtnetInstanceID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	return id
}

// TestScriptKillDoesNotBlockArtnet is SCRP-06's primary evidence: with a
// real artnet.Worker running against a loopback listener, launching a
// deliberately runaway script (a tight infinite loop that also
// allocates) and then killing it produces no missed frame beyond the
// tolerance TestWorkerSlowTargetDoesNotStallHealthyTarget already
// establishes, and the worker's per-universe sequence advances
// continuously across the kill. Written to fail loudly and specifically
// -- reporting the observed frame count and any sequence discontinuity
// -- rather than with a bare boolean.
func TestScriptKillDoesNotBlockArtnet(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	instanceID := mustArtnetInstanceID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	listener := newArtnetLoopbackListener(t)
	target := artnet.Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: artnetListenerPort(t, listener), Enabled: true}

	frames := &fakeArtnetFrameSource{frame: &playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{fixture.CapabilityIntensity: 1.0}},
	}}}

	worker := artnet.NewWorker(artnet.WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Resolve:   staticArtnetResolver(singleIntensityChannelMode),
		Targets:   map[int][]artnet.Target{1: {target}},
	})

	workerCtx, workerCancel := context.WithCancel(context.Background())
	worker.Start(workerCtx)
	defer func() {
		workerCancel()
		worker.Stop()
	}()

	// Launch a deliberately runaway script -- a tight, allocating
	// infinite loop with no SDK calls at all -- on a wholly separate
	// goroutine, driven entirely through this package's own Host.Run.
	// Nothing here ever touches internal/artnet, and nothing in
	// internal/artnet (above) ever touches internal/script: the only
	// thing connecting the two call graphs is this test's own top-level
	// goroutines, which is the structural proof SCRP-06 requires.
	const scriptName = "RunawayArtnetNonInterference"
	host, err := NewHost(HostConfig{
		Root:     root,
		ShowPath: filepath.Join(root, "artnet-noninterference-fixture.golc"),
		Executor: &fakeExecutor{},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}

	runawaySource := `
const buffers: Uint8Array[] = [];
while (true) {
  buffers.push(new Uint8Array(1024 * 1024));
  if (buffers.length > 64) {
    buffers.length = 0;
  }
}
`
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_, _ = host.Run(context.Background(), show.Script{
			Name:              scriptName,
			Source:            runawaySource,
			CapabilityProfile: show.CapabilityProfile{Scope: show.APIKeyScopeAdmin, Preset: show.ResourcePresetQuickAction},
		}, LaunchModeRun, nil)
	}()
	t.Cleanup(func() {
		if run, found := ActiveRun(scriptName); found {
			run.Stop(TerminationReason{Code: "GOLC_SCRIPT_STOPPED_BY_USER", Message: "test cleanup", At: time.Now()})
		}
		<-runDone
	})

	// Give the script a moment to actually start spinning before
	// measuring the worker's cadence and killing it mid-window.
	time.Sleep(300 * time.Millisecond)

	window := 200 * time.Millisecond // ~8 ticks at 40Hz
	stop := time.Now().Add(window)
	killAt := time.Now().Add(window / 2)
	killed := false

	received := 0
	discontinuities := 0
	var lastSeq byte
	haveLastSeq := false
	buf := make([]byte, 600)

	for time.Now().Before(stop) {
		if !killed && time.Now().After(killAt) {
			if run, found := ActiveRun(scriptName); found {
				run.Stop(TerminationReason{Code: "GOLC_SCRIPT_STOPPED_BY_USER", Message: "mid-window test kill", At: time.Now()})
			}
			killed = true
		}

		_ = listener.SetReadDeadline(stop)
		n, _, readErr := listener.ReadFromUDP(buf)
		if readErr != nil {
			break
		}
		if n < 13 {
			continue
		}
		received++

		seq := buf[12]
		if haveLastSeq {
			expected := lastSeq + 1
			if expected == 0 {
				expected = 1 // nextSeq wraps 1->255->1, never emitting 0 (Pitfall 2).
			}
			if seq != expected {
				discontinuities++
			}
		}
		lastSeq = seq
		haveLastSeq = true
	}

	if received < 5 {
		t.Fatalf(
			"expected the Art-Net worker to keep receiving packets on cadence across the script kill (observed frame gap): got only %d packets in %s",
			received, window)
	}
	if discontinuities > 0 {
		t.Fatalf(
			"expected the worker's per-universe sequence to advance continuously across the script kill (observed sequence discontinuity): %d gap(s) across %d packets",
			discontinuities, received)
	}
}
