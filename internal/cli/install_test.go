package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const startupHint = "run arch-caffeinate startup install to start it at login\n"

func TestInstallDoesNotLoad(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	opts := Options{
		Stdout: &stdout,
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
	if got := stdout.String(); got != startupHint {
		t.Fatalf("stdout %q", got)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
	binPath := filepath.Join(home, ".local", "bin", "arch-caffeinate")
	first := mustRead(t, plistPath)
	if got := mustArgs(t, first); !slices.Equal(got, []string{binPath, "run", "--idle-seconds", "5"}) {
		t.Fatalf("args %v", got)
	}
	runAtLoad, keepAlive := plistBools(t, first)
	if !runAtLoad || !keepAlive {
		t.Fatalf("RunAtLoad %v KeepAlive %v", runAtLoad, keepAlive)
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
	if got := mustArgs(t, dropped); !slices.Equal(got, []string{binPath, "run"}) {
		t.Fatalf("args %v", got)
	}
	if len(calls) != 0 {
		t.Fatalf("install called launchctl %v", calls)
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
	}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls %v", calls)
	}
}

func TestInstallStartupFlag(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
	var calls []string
	var stdout bytes.Buffer
	opts := Options{
		Stdout:     &stdout,
		Stderr:     &bytes.Buffer{},
		Getenv:     func(string) string { return home },
		Executable: exe,
		UID:        501,
		Launch: func(ctx context.Context, args ...string) ([]byte, error) {
			calls = append(calls, joinArgs(args))
			return []byte("ok\n"), nil
		},
	}
	if err := Run(context.Background(), []string{"install", "--startup", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q", stdout.String())
	}
	first := mustRead(t, plistPath)
	if err := Run(context.Background(), []string{"install", "--startup", "--idle-seconds", "5"}, opts); err != nil {
		t.Fatal(err)
	}
	if first != mustRead(t, plistPath) {
		t.Fatal("plist changed")
	}
	want := []string{
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls %v", calls)
	}
}

func TestStartupWithoutPlist(t *testing.T) {
	home := t.TempDir()
	var calls []string
	opts := Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Getenv: func(string) string { return home },
		UID:    501,
		Launch: func(ctx context.Context, args ...string) ([]byte, error) {
			calls = append(calls, joinArgs(args))
			return []byte("ok\n"), nil
		},
	}
	err := Run(context.Background(), []string{"startup", "install"}, opts)
	if err == nil || err.Error() != "plist missing, run install first" {
		t.Fatalf("err %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("calls %v", calls)
	}
	for _, args := range [][]string{
		{"startup"},
		{"startup", "nope"},
		{"startup", "install", "remove"},
	} {
		err := Run(context.Background(), args, opts)
		if err == nil || err.Error() != "usage: arch-caffeinate startup <install|remove>" {
			t.Fatalf("args %v err %v", args, err)
		}
	}
	if err := Run(context.Background(), []string{"startup", "remove"}, opts); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls %v", calls)
	}
}

func TestStartupInstallAndRemove(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "self")
	if err := os.WriteFile(exe, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.github.goddyb.arch-caffeinate.plist")
	var calls []string
	opts := Options{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Getenv:     func(string) string { return home },
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
	if err := Run(context.Background(), []string{"install"}, opts); err != nil {
		t.Fatal(err)
	}
	calls = nil
	if err := Run(context.Background(), []string{"startup", "install"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"startup", "install"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"startup", "remove"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"startup", "remove"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"uninstall"}, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatal("plist still present")
	}
	want := []string{
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootstrap gui/501 " + plistPath,
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
		"bootout gui/501/io.github.goddyb.arch-caffeinate",
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls %v", calls)
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func plistBools(t *testing.T, body string) (runAtLoad bool, keepAlive bool) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(body))
	var key string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return runAtLoad, keepAlive
		}
		if err != nil {
			t.Fatal(err)
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch el.Name.Local {
		case "key":
			if err := dec.DecodeElement(&key, &el); err != nil {
				t.Fatal(err)
			}
		case "true":
			switch key {
			case "RunAtLoad":
				runAtLoad = true
			case "KeepAlive":
				keepAlive = true
			}
			key = ""
		}
	}
}

func mustArgs(t *testing.T, body string) []string {
	t.Helper()
	args, err := programArguments([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return args
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
