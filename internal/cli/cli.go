package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
	"github.com/GoddyB/arch-caffeinate/internal/macos"
)

const label = "io.github.goddyb.arch-caffeinate"

type Options struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Getenv     func(string) string
	Executable string
	UID        int
	Now        func() time.Time
	Host       macos.Host
	Launch     func(ctx context.Context, args ...string) error
}

func Main(args []string) int {
	opts := Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
		UID:    os.Getuid(),
		Now:    time.Now,
		Host:   &macos.System{},
	}
	if exe, err := os.Executable(); err == nil {
		opts.Executable = exe
	}
	opts.Launch = func(ctx context.Context, launchArgs ...string) error {
		bin := opts.Getenv("ARCH_CAFFEINATE_LAUNCHCTL")
		if bin == "" {
			bin = "launchctl"
		}
		cmd := exec.CommandContext(ctx, bin, launchArgs...)
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
		return cmd.Run()
	}
	if err := Run(context.Background(), args, opts); err != nil {
		if !errors.Is(err, errDoctor) {
			fmt.Fprintln(opts.Stderr, err)
		}
		return 1
	}
	return 0
}

func Run(ctx context.Context, args []string, opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.UID == 0 {
		opts.UID = os.Getuid()
	}
	home, err := homeDir(opts)
	if err != nil {
		return err
	}
	paths := daemon.Paths{Home: home}
	if len(args) == 0 {
		return fmt.Errorf("usage: arch-caffeinate <install|uninstall|start|stop|run|status|doctor|wake|--version>")
	}
	cmd := args[0]
	rest := args[1:]
	if cmd == "--version" || cmd == "-version" {
		fmt.Fprintln(opts.Stdout, daemon.Version)
		return nil
	}
	switch cmd {
	case "install":
		idle, err := optionalIdle(rest)
		if err != nil {
			return err
		}
		return install(ctx, opts, paths, idle)
	case "uninstall":
		return uninstall(ctx, opts, paths)
	case "start":
		return start(ctx, opts, paths)
	case "stop":
		return stop(ctx, opts, paths)
	case "run":
		idle, poll, err := runFlags(rest)
		if err != nil {
			return err
		}
		host := opts.Host
		if host == nil {
			host = &macos.System{}
		}
		cfg := daemon.Config{IdleThreshold: time.Duration(idle) * time.Second, Poll: time.Duration(poll) * time.Millisecond, Now: opts.Now}
		cfg.Logf = func(format string, args ...any) {
			daemon.AppendLog(paths, fmt.Sprintf(format, args...))
		}
		return daemon.Run(ctx, host, paths, cfg)
	case "status":
		asJSON := false
		for _, a := range rest {
			if a == "--json" {
				asJSON = true
			} else {
				return fmt.Errorf("unknown flag %s", a)
			}
		}
		return status(ctx, opts, paths, asJSON)
	case "doctor":
		return doctor(ctx, opts, paths)
	case "wake":
		host := opts.Host
		if host == nil {
			host = &macos.System{}
		}
		return host.DeclareActivity(ctx)
	default:
		return fmt.Errorf("unknown command %s", cmd)
	}
}

func homeDir(opts Options) (string, error) {
	if h := opts.Getenv("HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

func optionalIdle(args []string) (int, error) {
	idle := 0
	seen := false
	for i := 0; i < len(args); i++ {
		if args[i] != "--idle-seconds" {
			return 0, fmt.Errorf("unknown flag %s", args[i])
		}
		i++
		if i >= len(args) {
			return 0, fmt.Errorf("--idle-seconds needs a value")
		}
		n, err := strconv.Atoi(args[i])
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--idle-seconds needs a positive integer")
		}
		idle = n
		seen = true
	}
	if !seen {
		return 0, nil
	}
	return idle, nil
}

func runFlags(args []string) (idle int, poll int, err error) {
	idle = 600
	poll = 1000
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--idle-seconds":
			i++
			if i >= len(args) {
				return 0, 0, fmt.Errorf("--idle-seconds needs a value")
			}
			idle, err = strconv.Atoi(args[i])
			if err != nil || idle <= 0 {
				return 0, 0, fmt.Errorf("--idle-seconds needs a positive integer")
			}
		case "--poll-ms":
			i++
			if i >= len(args) {
				return 0, 0, fmt.Errorf("--poll-ms needs a value")
			}
			poll, err = strconv.Atoi(args[i])
			if err != nil || poll <= 0 {
				return 0, 0, fmt.Errorf("--poll-ms needs a positive integer")
			}
		default:
			return 0, 0, fmt.Errorf("unknown flag %s", args[i])
		}
	}
	return idle, poll, nil
}

func install(ctx context.Context, opts Options, paths daemon.Paths, idle int) error {
	exe := opts.Executable
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if err := copyFile(exe, paths.Bin()); err != nil {
		return err
	}
	if err := writePlist(paths, idle); err != nil {
		return err
	}
	_ = launch(ctx, opts, "bootout", fmt.Sprintf("gui/%d/%s", opts.UID, label))
	return launch(ctx, opts, "bootstrap", fmt.Sprintf("gui/%d", opts.UID), paths.Plist())
}

func uninstall(ctx context.Context, opts Options, paths daemon.Paths) error {
	_ = launch(ctx, opts, "bootout", fmt.Sprintf("gui/%d/%s", opts.UID, label))
	if err := os.Remove(paths.Plist()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func start(ctx context.Context, opts Options, paths daemon.Paths) error {
	if _, err := os.Stat(paths.Plist()); err != nil {
		return fmt.Errorf("plist missing, run install first")
	}
	_ = launch(ctx, opts, "bootout", fmt.Sprintf("gui/%d/%s", opts.UID, label))
	return launch(ctx, opts, "bootstrap", fmt.Sprintf("gui/%d", opts.UID), paths.Plist())
}

func stop(ctx context.Context, opts Options, paths daemon.Paths) error {
	return launch(ctx, opts, "bootout", fmt.Sprintf("gui/%d/%s", opts.UID, label))
}

func launch(ctx context.Context, opts Options, args ...string) error {
	if opts.Launch != nil {
		return opts.Launch(ctx, args...)
	}
	bin := "launchctl"
	if opts.Getenv != nil && opts.Getenv("ARCH_CAFFEINATE_LAUNCHCTL") != "" {
		bin = opts.Getenv("ARCH_CAFFEINATE_LAUNCHCTL")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	return cmd.Run()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func writePlist(paths daemon.Paths, idle int) error {
	if err := os.MkdirAll(filepath.Dir(paths.Plist()), 0o755); err != nil {
		return err
	}
	args := []string{paths.Bin(), "run"}
	if idle > 0 {
		args = append(args, "--idle-seconds", strconv.Itoa(idle))
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0"><dict>` + "\n")
	b.WriteString("<key>Label</key><string>" + label + "</string>\n")
	b.WriteString("<key>ProgramArguments</key><array>\n")
	for _, a := range args {
		b.WriteString("<string>" + xmlEscape(a) + "</string>\n")
	}
	b.WriteString("</array>\n")
	b.WriteString("<key>RunAtLoad</key><true/>\n")
	b.WriteString("<key>KeepAlive</key><true/>\n")
	b.WriteString("</dict></plist>\n")
	return os.WriteFile(paths.Plist(), []byte(b.String()), 0o644)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func status(ctx context.Context, opts Options, paths daemon.Paths, asJSON bool) error {
	host := opts.Host
	if host == nil {
		host = &macos.System{}
	}
	saved, _ := daemon.ReadStatus(paths)
	power := "unknown"
	if p, err := host.Power(ctx); err == nil {
		power = p.String()
	} else if saved.Power != "" {
		power = saved.Power
	}
	idle := saved.IdleSeconds
	if d, err := host.Idle(ctx); err == nil {
		idle = d.Seconds()
	}
	display := saved.Display
	if display == "" {
		display = "on"
	}
	if d, ok, err := host.Display(ctx); err == nil && ok {
		display = d.String()
	}
	lock := "unknown"
	if s, err := host.ScreenLock(ctx); err == nil && s != "" {
		lock = s
	} else if saved.ScreenLock != "" {
		lock = saved.ScreenLock
	}
	threshold := saved.IdleThresholdSeconds
	if threshold == 0 {
		threshold = plistIdle(paths)
	}
	if threshold == 0 {
		threshold = 600
	}
	var pid *int
	running := false
	if saved.PID != nil && alive(*saved.PID) {
		pid = saved.PID
		running = true
	}
	sleep := saved.SleepPrevented && running
	st := daemon.Status{
		Running:              running,
		PID:                  pid,
		Power:                power,
		SleepPrevented:       sleep,
		Display:              display,
		IdleSeconds:          idle,
		IdleThresholdSeconds: threshold,
		ScreenLock:           lock,
		Version:              daemon.Version,
	}
	if asJSON {
		b, err := json.Marshal(st)
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.Stdout, "%s\n", b)
		return nil
	}
	pidText := "null"
	if pid != nil {
		pidText = strconv.Itoa(*pid)
	}
	fmt.Fprintf(opts.Stdout, "running: %t\npid: %s\npower: %s\nsleepPrevented: %t\ndisplay: %s\nidleSeconds: %g\nidleThresholdSeconds: %d\nscreenLock: %s\nversion: %s\n",
		running, pidText, power, sleep, display, idle, threshold, lock, daemon.Version)
	return nil
}

func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func plistIdle(paths daemon.Paths) int {
	b, err := os.ReadFile(paths.Plist())
	if err != nil {
		return 0
	}
	text := string(b)
	i := strings.Index(text, "--idle-seconds")
	if i < 0 {
		return 600
	}
	rest := text[i:]
	parts := strings.Split(rest, "<string>")
	if len(parts) < 2 {
		return 600
	}
	n := strings.SplitN(parts[1], "</string>", 2)[0]
	v, err := strconv.Atoi(n)
	if err != nil {
		return 600
	}
	return v
}

func doctor(ctx context.Context, opts Options, paths daemon.Paths) error {
	fail := 0
	line := func(level, msg string) {
		fmt.Fprintf(opts.Stdout, "%s %s\n", level, msg)
		if level == "FAIL" {
			fail++
		}
	}
	if goos() != "darwin" {
		line("FAIL", "macOS: not macOS")
	} else {
		line("PASS", "macOS")
	}
	bin, err := loginBin()
	if err != nil || bin == "" {
		line("FAIL", "binary: arch-caffeinate is not on PATH in a login bash")
	} else {
		line("PASS", "binary: "+bin)
	}
	if err := launchQuiet(ctx, opts, "print", fmt.Sprintf("gui/%d/%s", opts.UID, label)); err != nil {
		line("FAIL", "launchagent: not loaded")
	} else {
		line("PASS", "launchagent: loaded")
	}
	saved, err := daemon.ReadStatus(paths)
	poll := saved.PollMs
	if poll <= 0 {
		poll = 1000
	}
	fresh := err == nil && saved.WrittenAt > 0 && opts.Now().UnixMilli()-saved.WrittenAt <= int64(3*poll)
	if fresh && saved.PID != nil && alive(*saved.PID) {
		line("PASS", "heartbeat: fresh")
	} else {
		line("FAIL", "heartbeat: stale or missing")
	}
	host := opts.Host
	if host == nil {
		host = &macos.System{}
	}
	if _, err := host.Power(ctx); err != nil {
		line("FAIL", "power: not readable")
	} else {
		line("PASS", "power: readable")
	}
	if _, err := host.Idle(ctx); err != nil {
		line("FAIL", "idle: not readable")
	} else {
		line("PASS", "idle: readable")
	}
	lock, err := host.ScreenLock(ctx)
	switch {
	case err != nil:
		line("FAIL", "screenlock: not readable")
	case lock == "unknown" || lock == "":
		line("WARN", "screenlock: unknown")
	default:
		line("PASS", "screenlock: "+lock)
	}
	if fail > 0 {
		return errDoctor
	}
	return nil
}

var errDoctor = errors.New("doctor failed")

func loginBin() (string, error) {
	cmd := exec.Command("bash", "-lc", "command -v arch-caffeinate")
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func goos() string {
	if v := os.Getenv("ARCH_CAFFEINATE_GOOS"); v != "" {
		return v
	}
	return runtime.GOOS
}

func launchQuiet(ctx context.Context, opts Options, args ...string) error {
	if opts.Launch != nil {
		return opts.Launch(ctx, args...)
	}
	bin := "launchctl"
	if opts.Getenv != nil && opts.Getenv("ARCH_CAFFEINATE_LAUNCHCTL") != "" {
		bin = opts.Getenv("ARCH_CAFFEINATE_LAUNCHCTL")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	return cmd.Run()
}
