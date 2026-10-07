package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInstallPlistAndLaunch(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	var stderr bytes.Buffer
	opts := Options{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Getenv: func(k string) string {
			if k == "HOME" {
				return home
			}
			return ""
		},
		Executable: exe,
		UID:        501,
		Launch: func(ctx context.Context, args ...string) ([]byte, error) {
			calls = append(calls, joinArgs(args))
			if args[0] == "bootout" {
				return []byte("Boot-out failed: 3: No such process\n"), errors.New("exit status 3")
			}
			return []byte("bootstrap ok\n"), nil
		},
	}
	if err := Run(context.Background(), []string{"install", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr %q", stderr.String())
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
	binPath := filepath.Join(home, ".local", "bin", "arch-caffeinate")
	first := mustRead(t, plistPath)
	want := plistLiteral(binPath, true)
	if first != want {
		t.Fatalf("plist\n%s\nwant\n%s", first, want)
	}
	if mustRead(t, binPath) != "payload" {
		t.Fatal("binary")
	}
	if err := Run(context.Background(), []string{"install", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	second := mustRead(t, plistPath)
	if first != second {
		t.Fatalf("plist changed\n%s\n%s", first, second)
	}
	if err := Run(context.Background(), []string{"install"}, opts); err != nil {
		t.Fatal(err)
	}
	dropped := mustRead(t, plistPath)
	if dropped != plistLiteral(binPath, false) {
		t.Fatalf("plist\n%s", dropped)
	}
	if err := Run(context.Background(), []string{"uninstall"}, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatal("plist still present")
	}
	if mustRead(t, binPath) != "payload" {
		t.Fatal("uninstall removed the binary")
	}
	wantCalls := []string{
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls %v", calls)
	}
}

func TestStartStop(t *testing.T) {
	home := t.TempDir()
	var calls []string
	opts := Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		UID:    501,
		Launch: func(ctx context.Context, args ...string) ([]byte, error) {
			calls = append(calls, joinArgs(args))
			return nil, nil
		},
	}
	err := Run(context.Background(), []string{"start"}, opts)
	if err == nil || err.Error() != "plist missing, run install first" {
		t.Fatalf("err %v", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistPath, []byte("<plist></plist>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"start"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"stop"}, opts); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls %v", calls)
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func plistLiteral(bin string, idle bool) string {
	args := "<string>" + bin + "</string>\n<string>run</string>\n"
	if idle {
		args += "<string>--idle-seconds</string>\n<string>5</string>\n"
	}
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
		"<plist version=\"1.0\"><dict><key>Label</key><string>io.github.goddyb.arch-caffeinate</string><key>ProgramArguments</key><array>\n" +
		args +
		"</array><key>RunAtLoad</key><true></true><key>KeepAlive</key><true></true></dict></plist>\n"
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
