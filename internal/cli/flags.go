package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

type positiveInt struct {
	value int
	set   bool
}

func (p *positiveInt) String() string {
	return strconv.Itoa(p.value)
}

func (p *positiveInt) Set(raw string) error {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fmt.Errorf("needs a positive integer")
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
	idleFlag := &positiveInt{value: daemon.DefaultIdleSeconds}
	pollFlag := &positiveInt{value: daemon.DefaultPollMs}
	err = parseFlags("run", args, func(fs *flag.FlagSet) {
		fs.Var(idleFlag, "idle-seconds", "")
		fs.Var(pollFlag, "poll-ms", "")
	})
	if err != nil {
		return 0, 0, err
	}
	return idleFlag.value, pollFlag.value, nil
}

func parseInstallArgs(args []string) (*int, bool, error) {
	idleFlag := &positiveInt{}
	startup := false
	err := parseFlags("install", args, func(fs *flag.FlagSet) {
		fs.Var(idleFlag, "idle-seconds", "")
		fs.BoolVar(&startup, "startup", false, "")
	})
	if err != nil {
		return nil, false, err
	}
	if !idleFlag.set {
		return nil, startup, nil
	}
	value := idleFlag.value
	return &value, startup, nil
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
