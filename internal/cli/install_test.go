package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/core"
	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func TestInstallIsIdempotent(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	opts := Options{
		Stdout: discard(),
		Stderr: discard(),
		Getenv: func(k string) string {
			if k == "HOME" {
				return home
			}
			return ""
		},
		Executable: exe,
		UID:        501,
		Launch: func(ctx context.Context, args ...string) error {
			calls = append(calls, strings.Join(args, " "))
			return nil
		},
	}
	if err := Run(context.Background(), []string{"install", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	plist := mustRead(t, filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist"))
	if !strings.Contains(plist, "--idle-seconds") || !strings.Contains(plist, ">5<") || !strings.Contains(plist, ">run<") {
		t.Fatalf("plist %s", plist)
	}
	if !strings.Contains(plist, "<key>RunAtLoad</key><true/>") || !strings.Contains(plist, "<key>KeepAlive</key><true/>") {
		t.Fatalf("plist flags %s", plist)
	}
	bin := mustRead(t, filepath.Join(home, ".local", "bin", "arch-caffeinate"))
	if bin != "payload" {
		t.Fatalf("binary %q", bin)
	}
	if err := Run(context.Background(), []string{"install", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"install"}, opts); err != nil {
		t.Fatal(err)
	}
	plist = mustRead(t, filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist"))
	if strings.Contains(plist, "idle-seconds") {
		t.Fatalf("latest install dropped the flag, plist %s", plist)
	}
	bootstraps := 0
	for _, c := range calls {
		if strings.HasPrefix(c, "bootstrap gui/501 ") {
			bootstraps++
		}
	}
	if bootstraps != 3 {
		t.Fatalf("calls %v", calls)
	}
	if err := Run(context.Background(), []string{"uninstall"}, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")); !os.IsNotExist(err) {
		t.Fatal("plist still present")
	}
	if mustRead(t, filepath.Join(home, ".local", "bin", "arch-caffeinate")) != "payload" {
		t.Fatal("uninstall removed the binary")
	}
}

type okHost struct{}

func (okHost) Power(context.Context) (core.Power, error) { return core.PowerAC, nil }
func (okHost) Idle(context.Context) (time.Duration, error) {
	return time.Second, nil
}
func (okHost) ScreenLock(context.Context) (string, error) { return "off", nil }
func (okHost) Display(context.Context) (core.Display, bool, error) {
	return core.DisplayOn, true, nil
}
func (okHost) NotificationPending() bool                { return false }
func (okHost) ClearNotification()                       {}
func (okHost) AcquireSleep(context.Context) error       { return nil }
func (okHost) ReleaseSleep() error                      { return nil }
func (okHost) DisplayOff(context.Context) error         { return nil }
func (okHost) DeclareActivity(context.Context) error    { return nil }
func (okHost) StartNotifications(context.Context) error { return nil }

func TestDoctorNotMacOS(t *testing.T) {
	home := t.TempDir()
	var buf strings.Builder
	opts := Options{
		Stdout: &buf,
		Stderr: discard(),
		Getenv: func(string) string { return home },
		UID:    501,
		Now:    time.Now,
		Host:   okHost{},
		Launch: func(context.Context, ...string) error { return os.ErrNotExist },
	}
	err := Run(context.Background(), []string{"doctor"}, opts)
	if err == nil {
		t.Fatal("expected failure")
	}
	out := buf.String()
	if !strings.Contains(out, "FAIL macOS: not macOS") {
		t.Fatalf("output %s", out)
	}
}

func TestVersion(t *testing.T) {
	var buf strings.Builder
	opts := Options{Stdout: &buf, Stderr: discard(), Getenv: func(string) string { return t.TempDir() }}
	if err := Run(context.Background(), []string{"--version"}, opts); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != daemon.Version {
		t.Fatalf("version %q", buf.String())
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type sink struct{}

func (sink) Write(p []byte) (int, error) { return len(p), nil }

func discard() sink { return sink{} }
