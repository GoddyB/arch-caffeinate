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
