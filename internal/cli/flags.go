package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

type positiveInt struct {
	name  string
	value int
	set   bool
}

func (p *positiveInt) String() string {
	return strconv.Itoa(p.value)
}

func (p *positiveInt) Set(raw string) error {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fmt.Errorf("--%s needs a positive integer", p.name)
	}
	p.value = n
	p.set = true
	return nil
}

func parseFlags(name string, args []string, define func(*flag.FlagSet)) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	define(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %s", fs.Arg(0))
	}
	return nil
}

func parseRunArgs(args []string) (idle int, poll int, err error) {
	idleFlag := &positiveInt{name: "idle-seconds", value: daemon.DefaultIdleSeconds}
	pollFlag := &positiveInt{name: "poll-ms", value: daemon.DefaultPollMs}
	err = parseFlags("run", args, func(fs *flag.FlagSet) {
		fs.Var(idleFlag, "idle-seconds", "")
		fs.Var(pollFlag, "poll-ms", "")
	})
	if err != nil {
		return 0, 0, err
	}
	return idleFlag.value, pollFlag.value, nil
}

func parseInstallArgs(args []string) (idle int, set bool, err error) {
	idleFlag := &positiveInt{name: "idle-seconds"}
	err = parseFlags("install", args, func(fs *flag.FlagSet) {
		fs.Var(idleFlag, "idle-seconds", "")
	})
	if err != nil {
		return 0, false, err
	}
	return idleFlag.value, idleFlag.set, nil
}

func parseStatusArgs(args []string) (bool, error) {
	asJSON := false
	err := parseFlags("status", args, func(fs *flag.FlagSet) {
		fs.BoolVar(&asJSON, "json", false, "")
	})
	if err != nil {
		return false, err
	}
	return asJSON, nil
}
