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

func install(ctx context.Context, opts Options, paths daemon.Paths, idle int, set bool) error {
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
	args := []string{paths.Bin(), "run"}
	if set {
		args = append(args, "--idle-seconds", strconv.Itoa(idle))
	}
	body, err := encodePlist(args)
	if err != nil {
		return err
	}
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

func encodePlist(args []string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "plist"}, Attr: []xml.Attr{{Name: xml.Name{Local: "version"}, Value: "1.0"}}}); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return nil, err
	}
	if err := writeKV(enc, "Label", daemon.Label); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.CharData("ProgramArguments")); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "key"}}); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "array"}}); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.CharData("\n")); err != nil {
		return nil, err
	}
	for _, arg := range args {
		if err := writeString(enc, arg); err != nil {
			return nil, err
		}
		if err := enc.EncodeToken(xml.CharData("\n")); err != nil {
			return nil, err
		}
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "array"}}); err != nil {
		return nil, err
	}
	if err := writeBool(enc, "RunAtLoad"); err != nil {
		return nil, err
	}
	if err := writeBool(enc, "KeepAlive"); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "plist"}}); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeKV(enc *xml.Encoder, key, value string) error {
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData(key)); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "key"}}); err != nil {
		return err
	}
	return writeString(enc, value)
}

func writeString(enc *xml.Encoder, value string) error {
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "string"}}); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData(value)); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "string"}})
}

func writeBool(enc *xml.Encoder, key string) error {
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData(key)); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "key"}}); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "true"}}); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "true"}})
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
	args, err := programArguments(b)
	if err != nil || len(args) < 2 {
		return idle
	}
	parsed, _, err := parseRunArgs(args[2:])
	if err != nil {
		return idle
	}
	return parsed
}
