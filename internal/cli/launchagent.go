package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/GoddyB/arch-caffeinate/internal/daemon"
)

func serviceTarget(uid int) string {
	return fmt.Sprintf("gui/%d/%s", uid, daemon.Label)
}

func domainTarget(uid int) string {
	return fmt.Sprintf("gui/%d", uid)
}

func install(ctx context.Context, opts Options, paths daemon.Paths, idle *int) error {
	exe := opts.Executable
	if exe == "" {
		return fmt.Errorf("executable path is unknown")
	}
	if err := copyFile(exe, paths.Bin()); err != nil {
		return err
	}
	body := encodePlist(agentArgs(paths.Bin(), idle))
	if err := daemon.WriteFileAtomic(paths.Plist(), body, 0o644); err != nil {
		return err
	}
	return reload(ctx, opts, paths)
}

func uninstall(ctx context.Context, opts Options, paths daemon.Paths) error {
	_, _ = opts.Launch(ctx, "bootout", serviceTarget(opts.UID))
	if err := os.Remove(paths.Plist()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func start(ctx context.Context, opts Options, paths daemon.Paths) error {
	if _, err := os.Stat(paths.Plist()); err != nil {
		return fmt.Errorf("plist missing, run install first")
	}
	return reload(ctx, opts, paths)
}

func stop(ctx context.Context, opts Options, paths daemon.Paths) error {
	return runLaunch(ctx, opts, "bootout", serviceTarget(opts.UID))
}

func reload(ctx context.Context, opts Options, paths daemon.Paths) error {
	_, _ = opts.Launch(ctx, "bootout", serviceTarget(opts.UID))
	return runLaunch(ctx, opts, "bootstrap", domainTarget(opts.UID), paths.Plist())
}

func runLaunch(ctx context.Context, opts Options, args ...string) error {
	out, err := opts.Launch(ctx, args...)
	if err != nil {
		if len(out) > 0 {
			if _, werr := opts.Stderr.Write(out); werr != nil {
				return werr
			}
			if out[len(out)-1] != '\n' {
				fmt.Fprintln(opts.Stderr)
			}
		}
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	b, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return daemon.WriteFileAtomic(dst, b, 0o755)
}

func agentArgs(bin string, idle *int) []string {
	args := []string{bin, "run"}
	if idle != nil {
		args = append(args, "--idle-seconds", strconv.Itoa(*idle))
	}
	return args
}

func encodePlist(args []string) []byte {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buf.WriteString("<plist version=\"1.0\"><dict>\n<key>Label</key>")
	writeString(&buf, daemon.Label)
	buf.WriteString("<key>ProgramArguments</key><array>\n")
	for _, arg := range args {
		writeString(&buf, arg)
	}
	buf.WriteString("</array>\n<key>RunAtLoad</key><true/>\n<key>KeepAlive</key><true/>\n</dict></plist>\n")
	return buf.Bytes()
}

func writeString(buf *bytes.Buffer, value string) {
	buf.WriteString("<string>")
	_ = xml.EscapeText(buf, []byte(value))
	buf.WriteString("</string>\n")
}

func programArguments(data []byte) ([]string, error) {
	var doc struct {
		Array struct {
			Strings []string `xml:"string"`
		} `xml:"dict>array"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.Array.Strings, nil
}

func installedIdle(paths daemon.Paths) int {
	idle := daemon.DefaultIdleSeconds
	b, err := os.ReadFile(paths.Plist())
	if err != nil {
		return idle
	}
	parsed, ok := idleFromAgentArgs(b)
	if !ok {
		return idle
	}
	return parsed
}

func idleFromAgentArgs(data []byte) (int, bool) {
	args, err := programArguments(data)
	if err != nil || len(args) < 2 || args[1] != "run" {
		return 0, false
	}
	parsed, _, err := parseRunArgs(args[2:])
	if err != nil {
		return 0, false
	}
	return parsed, true
}
