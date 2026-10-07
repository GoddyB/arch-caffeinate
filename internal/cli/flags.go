package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func parseRunArgs(args []string) (idle int, poll int, err error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&idle, "idle-seconds", daemon.DefaultIdleSeconds, "")
	fs.IntVar(&poll, "poll-ms", daemon.DefaultPollMs, "")
	if err := fs.Parse(args); err != nil {
		return 0, 0, rewriteFlagErr(err)
	}
	if fs.NArg() != 0 {
		return 0, 0, fmt.Errorf("unknown flag %s", fs.Arg(0))
	}
	if idle <= 0 {
		return 0, 0, fmt.Errorf("--idle-seconds needs a positive integer")
	}
	if poll <= 0 {
		return 0, 0, fmt.Errorf("--poll-ms needs a positive integer")
	}
	return idle, poll, nil
}

func parseInstallArgs(args []string) (idle int, set bool, err error) {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&idle, "idle-seconds", 0, "")
	if err := fs.Parse(args); err != nil {
		return 0, false, rewriteFlagErr(err)
	}
	if fs.NArg() != 0 {
		return 0, false, fmt.Errorf("unknown flag %s", fs.Arg(0))
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "idle-seconds" {
			set = true
		}
	})
	if set && idle <= 0 {
		return 0, false, fmt.Errorf("--idle-seconds needs a positive integer")
	}
	return idle, set, nil
}

func parseStatusArgs(args []string) (bool, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := false
	fs.BoolVar(&asJSON, "json", false, "")
	if err := fs.Parse(args); err != nil {
		return false, rewriteFlagErr(err)
	}
	if fs.NArg() != 0 {
		return false, fmt.Errorf("unknown flag %s", fs.Arg(0))
	}
	return asJSON, nil
}

func rewriteFlagErr(err error) error {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "flag provided but not defined: "):
		name := strings.TrimLeft(strings.TrimPrefix(msg, "flag provided but not defined: "), "-")
		return fmt.Errorf("unknown flag --%s", name)
	case strings.Contains(msg, "needs an argument") && strings.Contains(msg, "idle-seconds"):
		return fmt.Errorf("--idle-seconds needs a value")
	case strings.Contains(msg, "needs an argument") && strings.Contains(msg, "poll-ms"):
		return fmt.Errorf("--poll-ms needs a value")
	case strings.Contains(msg, "invalid value") && strings.Contains(msg, "idle-seconds"):
		return fmt.Errorf("--idle-seconds needs a positive integer")
	case strings.Contains(msg, "invalid value") && strings.Contains(msg, "poll-ms"):
		return fmt.Errorf("--poll-ms needs a positive integer")
	default:
		return err
	}
}
