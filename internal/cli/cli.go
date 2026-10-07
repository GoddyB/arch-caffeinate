package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

type Options struct {
	Stdout      io.Writer
	Stderr      io.Writer
	Getenv      func(string) string
	Executable  string
	UID         int
	Now         func() time.Time
	Host        daemon.Host
	Launch      func(ctx context.Context, args ...string) ([]byte, error)
	GOOS        string
	LoginLookup func() (string, error)
}

func (o Options) withDefaults() Options {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.UID == 0 {
		o.UID = os.Getuid()
	}
	if o.Host == nil {
		o.Host = &macosSystem{}
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.LoginLookup == nil {
		o.LoginLookup = defaultLoginLookup
	}
	if o.Launch == nil {
		getenv := o.Getenv
		o.Launch = func(ctx context.Context, args ...string) ([]byte, error) {
			bin := "launchctl"
			if v := getenv("ARCH_CAFFEINATE_LAUNCHCTL"); v != "" {
				bin = v
			}
			return exec.CommandContext(ctx, bin, args...).CombinedOutput()
		}
	}
	return o
}

func defaultLoginLookup() (string, error) {
	b, err := exec.Command("bash", "-lc", "command -v arch-caffeinate").Output()
	return strings.TrimSpace(string(b)), err
}

func Main(args []string) int {
	opts := Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
		UID:    os.Getuid(),
		Now:    time.Now,
	}
	if exe, err := os.Executable(); err == nil {
		opts.Executable = exe
	}
	ctx, stop := signalNotifyContext()
	defer stop()
	if err := Run(ctx, args, opts); err != nil {
		if !errors.Is(err, errDoctor) {
			fmt.Fprintln(opts.Stderr, err)
		}
		return 1
	}
	return 0
}

func Run(ctx context.Context, args []string, opts Options) error {
	opts = opts.withDefaults()
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
		idle, set, err := parseInstallArgs(rest)
		if err != nil {
			return err
		}
		return install(ctx, opts, paths, idle, set)
	case "uninstall":
		return uninstall(ctx, opts, paths)
	case "start":
		return start(ctx, opts, paths)
	case "stop":
		return stop(ctx, opts, paths)
	case "run":
		idle, poll, err := parseRunArgs(rest)
		if err != nil {
			return err
		}
		cfg := daemon.Config{
			IdleThreshold: time.Duration(idle) * time.Second,
			Poll:          time.Duration(poll) * time.Millisecond,
			Now:           opts.Now,
			Logf: func(format string, args ...any) {
				daemon.AppendLog(paths, fmt.Sprintf(format, args...))
			},
		}
		return daemon.Run(ctx, opts.Host, paths, cfg)
	case "status":
		asJSON, err := parseStatusArgs(rest)
		if err != nil {
			return err
		}
		return status(ctx, opts, paths, asJSON)
	case "doctor":
		return doctor(ctx, opts, paths)
	case "wake":
		return opts.Host.DeclareActivity(ctx)
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
