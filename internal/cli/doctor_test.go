package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func TestDoctorNotMacOS(t *testing.T) {
	home := t.TempDir()
	now := time.UnixMilli(1_700_000_000_000)
	bin := filepath.Join(home, ".local", "bin", "arch-caffeinate")
	writeFreshState(t, home, now.Add(-time.Second))
	var buf strings.Builder
	opts := Options{
		Stdout: &buf,
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		UID:    501,
		Now:    func() time.Time { return now },
		GOOS:   "linux",
		LoginLookup: func() (string, error) {
			return bin, nil
		},
		Host: doctorHost{
			power:   core.PowerAC,
			idle:    time.Second,
			display: core.DisplayOn,
			lock:    core.ScreenLockImmediate,
		},
		Launch: func(context.Context, ...string) ([]byte, error) {
			return []byte("SERVICE DUMP\n"), nil
		},
	}
	err := Run(context.Background(), []string{"doctor"}, opts)
	if !errors.Is(err, errDoctor) {
		t.Fatalf("err %v", err)
	}
	want := "FAIL macOS: not macOS\n" +
		"PASS binary: " + bin + "\n" +
		"PASS launchagent: loaded\n" +
		"PASS heartbeat: fresh\n" +
		"PASS power: readable\n" +
		"PASS idle: readable\n" +
		"PASS screenlock: immediate\n"
	if buf.String() != want {
		t.Fatalf("output\n%s", buf.String())
	}
}

func TestDoctorWarnIsSuccess(t *testing.T) {
	home := t.TempDir()
	now := time.UnixMilli(1_700_000_000_000)
	bin := filepath.Join(home, "arch-caffeinate")
	writeFreshState(t, home, now.Add(-time.Second))
	var buf strings.Builder
	opts := doctorOpts(home, now, bin, &buf)
	opts.GOOS = "darwin"
	opts.Host = doctorHost{power: core.PowerAC, idle: time.Second, display: core.DisplayOn, lock: core.ScreenLockUnknown}
	if err := Run(context.Background(), []string{"doctor"}, opts); err != nil {
		t.Fatal(err)
	}
	want := "PASS macOS\n" +
		"PASS binary: " + bin + "\n" +
		"PASS launchagent: loaded\n" +
		"PASS heartbeat: fresh\n" +
		"PASS power: readable\n" +
		"PASS idle: readable\n" +
		"WARN screenlock: unknown\n"
	if buf.String() != want {
		t.Fatalf("output\n%s", buf.String())
	}
}

func TestDoctorHeartbeatBoundary(t *testing.T) {
	home := t.TempDir()
	now := time.UnixMilli(1_700_000_000_000)
	bin := filepath.Join(home, "arch-caffeinate")
	writeStateAt(t, home, now.UnixMilli()-3000)
	var buf strings.Builder
	opts := doctorOpts(home, now, bin, &buf)
	if err := Run(context.Background(), []string{"doctor"}, opts); err != nil {
		t.Fatalf("fresh boundary: %v\n%s", err, buf.String())
	}
	buf.Reset()
	writeStateAt(t, home, now.UnixMilli()-3001)
	err := Run(context.Background(), []string{"doctor"}, opts)
	if !errors.Is(err, errDoctor) {
		t.Fatal(err)
	}
	want := "PASS macOS\n" +
		"PASS binary: " + bin + "\n" +
		"PASS launchagent: loaded\n" +
		"FAIL heartbeat: stale or missing\n" +
		"PASS power: readable\n" +
		"PASS idle: readable\n" +
		"PASS screenlock: immediate\n"
	if buf.String() != want {
		t.Fatalf("output\n%s", buf.String())
	}
}

func TestVersionAndMain(t *testing.T) {
	home := t.TempDir()
	var ver bytes.Buffer
	opts := Options{
		Stdout: &ver,
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		Host:   doctorHost{power: core.PowerAC, idle: 3 * time.Second, display: core.DisplayOff, lock: core.ScreenLockImmediate},
		Now:    time.Now,
	}
	if err := Run(context.Background(), []string{"--version"}, opts); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\n$`).Match(ver.Bytes()) {
		t.Fatalf("version %q", ver.String())
	}
	var status bytes.Buffer
	opts.Stdout = &status
	if err := Run(context.Background(), []string{"status", "--json"}, opts); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(status.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != strings.TrimSpace(ver.String()) {
		t.Fatalf("status %s version %s", status.String(), ver.String())
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	os.Stderr = w
	code := Main([]string{"--version"})
	w.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || string(b) != ver.String() {
		t.Fatalf("main version code %d out %q", code, b)
	}
	code = Main([]string{"nope"})
	if code != 1 {
		t.Fatalf("main code %d", code)
	}
}

func doctorOpts(home string, now time.Time, bin string, buf *strings.Builder) Options {
	return Options{
		Stdout: buf,
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		UID:    501,
		Now:    func() time.Time { return now },
		GOOS:   "darwin",
		LoginLookup: func() (string, error) {
			return bin, nil
		},
		Host: doctorHost{power: core.PowerAC, idle: time.Second, display: core.DisplayOn, lock: core.ScreenLockImmediate},
		Launch: func(context.Context, ...string) ([]byte, error) {
			return []byte("SERVICE DUMP\n"), nil
		},
	}
}

func writeFreshState(t *testing.T, home string, at time.Time) {
	t.Helper()
	writeStateAt(t, home, at.UnixMilli())
}

func writeStateAt(t *testing.T, home string, written int64) {
	t.Helper()
	pid := os.Getpid()
	rec := daemon.StateFile{
		Status:    daemon.Status{PID: &pid},
		WrittenAt: written,
		PollMs:    1000,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	path := daemon.Paths{Home: home}.StateFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

type doctorHost struct {
	power      core.Power
	powerErr   error
	idle       time.Duration
	idleErr    error
	display    core.Display
	displayErr error
	lock       core.ScreenLock
	lockErr    error
	declareErr error
}

func (h doctorHost) Power(context.Context) (core.Power, error) {
	if h.powerErr != nil {
		return core.PowerUnknown, h.powerErr
	}
	return h.power, nil
}
func (h doctorHost) Idle(context.Context) (time.Duration, error) {
	if h.idleErr != nil {
		return 0, h.idleErr
	}
	return h.idle, nil
}
func (h doctorHost) Display(context.Context) (core.Display, error) {
	if h.displayErr != nil {
		return core.DisplayOn, h.displayErr
	}
	return h.display, nil
}
func (h doctorHost) ScreenLock(context.Context) (core.ScreenLock, error) {
	if h.lockErr != nil {
		return core.ScreenLockUnknown, h.lockErr
	}
	return h.lock, nil
}
func (h doctorHost) SleepHeld() bool                                                { return false }
func (h doctorHost) TakeNotification() bool                                         { return false }
func (h doctorHost) AcquireSleep(context.Context) error                             { return nil }
func (h doctorHost) ReleaseSleep(context.Context) error                             { return nil }
func (h doctorHost) DisplayOff(context.Context) error                               { return nil }
func (h doctorHost) DeclareActivity(context.Context) error                          { return h.declareErr }
func (h doctorHost) StartNotifications(context.Context, func(string, ...any)) error { return nil }
