package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func TestStatusJSON(t *testing.T) {
	home := t.TempDir()
	var ver bytes.Buffer
	base := statusOpts(home)
	base.Stdout = &ver
	if err := Run(context.Background(), []string{"--version"}, base); err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(ver.String())
	var buf bytes.Buffer
	opts := statusOpts(home)
	opts.Stdout = &buf
	if err := Run(context.Background(), []string{"status", "--json"}, opts); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("{\"running\":false,\"pid\":null,\"power\":\"ac\",\"sleepPrevented\":false,\"display\":\"off\",\"idleSeconds\":3,\"idleThresholdSeconds\":600,\"screenLock\":\"immediate\",\"version\":%q}\n", version)
	if buf.String() != want {
		t.Fatalf("json %s", buf.String())
	}
	buf.Reset()
	err := Run(context.Background(), []string{"status", "--nope"}, opts)
	if err == nil || err.Error() != "unknown flag --nope" {
		t.Fatalf("err %v", err)
	}
}

func TestStatusTextAndDeadDaemonThreshold(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := statusOpts(home)
	opts.Executable = exe
	opts.UID = 501
	opts.Launch = func(context.Context, ...string) ([]byte, error) { return nil, nil }
	if err := Run(context.Background(), []string{"install", "--idle-seconds", "7"}, opts); err != nil {
		t.Fatal(err)
	}
	dead := 1 << 30
	pid := dead
	rec := daemon.StateFile{
		Status: daemon.Status{
			Running:              true,
			PID:                  &pid,
			SleepPrevented:       true,
			IdleThresholdSeconds: 5,
			Power:                "battery",
			Display:              "on",
			ScreenLock:           "off",
		},
		WrittenAt: time.Now().UnixMilli(),
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
	var buf bytes.Buffer
	opts.Stdout = &buf
	if err := Run(context.Background(), []string{"status"}, opts); err != nil {
		t.Fatal(err)
	}
	var ver bytes.Buffer
	opts.Stdout = &ver
	if err := Run(context.Background(), []string{"--version"}, opts); err != nil {
		t.Fatal(err)
	}
	want := "running: false\npid: null\npower: ac\nsleepPrevented: false\ndisplay: off\nidleSeconds: 3\nidleThresholdSeconds: 7\nscreenLock: immediate\nversion: " + strings.TrimSpace(ver.String()) + "\n"
	if buf.String() != want {
		t.Fatalf("status\n%s", buf.String())
	}
}

func TestStatusRunningDaemon(t *testing.T) {
	home := t.TempDir()
	pid := os.Getpid()
	rec := daemon.StateFile{
		Status: daemon.Status{
			PID:                  &pid,
			SleepPrevented:       true,
			IdleThresholdSeconds: 9,
		},
		WrittenAt: time.Now().UnixMilli(),
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
	var buf bytes.Buffer
	opts := statusOpts(home)
	opts.Stdout = &buf
	if err := Run(context.Background(), []string{"status", "--json"}, opts); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["running"] != true || got["pid"] != float64(pid) || got["sleepPrevented"] != true || got["idleThresholdSeconds"] != float64(9) {
		t.Fatalf("%s", buf.String())
	}
}

func TestWakeAndRunFlags(t *testing.T) {
	home := t.TempDir()
	opts := statusOpts(home)
	opts.Host = doctorHost{declareErr: errors.New("declare failed")}
	err := Run(context.Background(), []string{"wake"}, opts)
	if err == nil || err.Error() != "declare failed" {
		t.Fatalf("err %v", err)
	}
	opts.Host = doctorHost{}
	if err := Run(context.Background(), []string{"wake"}, opts); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"run", "--idle-seconds", "0"},
		{"run", "--idle-seconds", "-3"},
		{"run", "--poll-ms", "0"},
		{"run", "--idle-seconds"},
	} {
		err := Run(context.Background(), args, opts)
		if err == nil {
			t.Fatalf("args %v", args)
		}
	}
	err = Run(context.Background(), []string{"run", "--idle-seconds", "0"}, opts)
	if err == nil || err.Error() != "--idle-seconds needs a positive integer" {
		t.Fatalf("err %v", err)
	}
	err = Run(context.Background(), []string{"run", "--idle-seconds"}, opts)
	if err == nil || err.Error() != "--idle-seconds needs a value" {
		t.Fatalf("err %v", err)
	}
}

func statusOpts(home string) Options {
	return Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		Host: doctorHost{
			power:   core.PowerAC,
			idle:    3 * time.Second,
			display: core.DisplayOff,
			lock:    core.ScreenLockImmediate,
		},
		Now: time.Now,
	}
}
