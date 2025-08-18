package sthpkgs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aottr/sth/internal/utils"
)

func ExecuteResolved(ctx context.Context, rr ResolveResult) error {
	// ensure all directories
	if err := os.MkdirAll(rr.Paths.CacheDir, 0o755); err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}
	if err := os.MkdirAll(rr.Paths.PkgsDir, 0o755); err != nil {
		return fmt.Errorf("pkgs dir: %w", err)
	}
	if err := os.MkdirAll(rr.Paths.BinDir, 0o755); err != nil {
		return fmt.Errorf("bin dir: %w", err)
	}

	// if symlink points to the target, we're done
	link := filepath.Join(rr.Paths.BinDir, rr.Resolved.BinName)
	if linksTo(link, rr.Resolved.BinaryPath) {
		if st, err := os.Stat(rr.Resolved.BinaryPath); err == nil && (st.Mode()&0o111) != 0 {
			fmt.Printf("[sth] ✅ already installed: %s -> %s\n", link, rr.Resolved.BinaryPath)
			checkPathHint(ctx, rr.Paths.BinDir)
			return nil
		}
	}

	for _, a := range rr.Actions {
		printStep(ctx, a, &rr)
		if err := execAction(ctx, a, rr); err != nil {
			return fmt.Errorf("%s: %w", a.Type, err)
		}
	}
	checkPathHint(ctx, rr.Paths.BinDir)
	return nil
}

var actionMsgs = map[string]struct{ icon, msg string }{
	"download": {"⬇️", "Downloading %s"},
	"verify":   {"✅", "Verifying checksum"},
	"mkdir":    {"", "Creating %s"},
	"move":     {"➡️", "Moving binary"},
	"gunzip":   {"📦", "Decompressing"},
	"extract":  {"📦", "Extracting archive"},
	"chmod":    {"🔑", "Changing mode %s %s"},
	"symlink":  {"🔗", "Linking %s -> %s"},
	"shell":    {"", "Running shell command"},
}

func printStep(ctx context.Context, a InstallAction, rr *ResolveResult) {
	opts := utils.GetExecOptions(ctx)
	if opts.Quiet {
		return
	}
	m, ok := actionMsgs[a.Type]
	if !ok {
		fmt.Printf("• %s\n", a.Type)
		return
	}
	switch a.Type {
	case "download":
		fmt.Printf("[sth] %s %s\n", m.icon, fmt.Sprintf(m.msg, rr.Resolved.URL))
	case "mkdir":
		fmt.Printf("[sth] %s %s\n", m.icon, fmt.Sprintf(m.msg, a.Args["path"]))
	case "chmod":
		fmt.Printf("[sth] %s %s\n", m.icon, fmt.Sprintf(m.msg, a.Args["mode"], a.Args["path"]))
	case "symlink":
		fmt.Printf("[sth] %s %s\n", m.icon, fmt.Sprintf(m.msg, a.Args["dest"], a.Args["src"]))
	case "shell":
		fmt.Printf("[sth] %s %s\n", m.icon, fmt.Sprintf(m.msg, a.Args["cmd"]))
	default:
		fmt.Printf("[sth] %s %s\n", m.icon, m.msg)
	}
}

func checkPathHint(ctx context.Context, binDir string) {
	opts := utils.GetExecOptions(ctx)
	if opts.Quiet {
		return
	}
	if !pathOnEnv(binDir, os.Getenv("PATH")) {
		fmt.Printf("\nNote: %s is not in your PATH.\n", binDir)
		fmt.Printf("Add this to your shell profile, then restart your shell:\n")
		fmt.Printf("  export PATH=\"%s:$PATH\"\n", binDir)
	}
}

func pathOnEnv(dir, envPath string) bool {
	if dir == "" || envPath == "" {
		return false
	}
	// Normalize for comparison
	dir = filepath.Clean(dir)
	sep := string(os.PathListSeparator)

	for p := range strings.SplitSeq(envPath, sep) {
		if p == "" {
			continue
		}
		if filepath.Clean(p) == dir {
			return true
		}
	}
	return false
}

func execAction(ctx context.Context, a InstallAction, rr ResolveResult) error {
	switch a.Type {
	case "download":
		return actionDownload(ctx, a.Args["url"], a.Args["dest"])
	case "verify":
		return actionVerify(a.Args["file"], a.Args["sha256"])
	case "mkdir":
		return os.MkdirAll(a.Args["path"], parseMode(a.Args["mode"], 0o755))
	case "move":
		return actionMove(a.Args["src"], a.Args["dest"])
	// case "gunzip":
	// 	return actionGunzip(a.Args["src"], a.Args["dest"], rr.Resolved.Mode)
	case "extract":
		return actionExtract(a.Args["src"], a.Args["dest"])
	case "chmod":
		return actionChmod(a.Args["path"], a.Args["mode"])
	case "symlink":
		return actionSymlink(a.Args["src"], a.Args["dest"])
	case "shell":
		return runShell(ctx, a.Args["cmd"], a.System)
	default:
		return fmt.Errorf("unknown action: %s", a.Type)
	}
}

func actionDownload(ctx context.Context, url, dest string) error {
	tmp := dest + ".part"
	if err := downloadToFile(ctx, url, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func runShell(ctx context.Context, cmdStr string, system bool) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	c := exec.CommandContext(ctx, shell, "-lc", cmdStr)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if !system {
		// user-space: do not elevate
	}
	return c.Run()
}

func actionVerify(file, sha string) error {
	got, err := fileSHA256(file)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, sha) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, sha)
	}
	return nil
}

func actionMove(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dest)
}

// func actionGunzip(src, dest, modeStr string) error {
// 	mode := parseMode(modeStr, 0o755)
// 	if err := gunzipFile(src, dest, mode); err != nil {
// 		return err
// 	}
// 	return nil
// }

func actionExtract(src, dest string) error {

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz"):
		return extractTarGz(src, dest)
	case strings.HasSuffix(src, ".zip"):
		return extractZip(src, dest)
	default:
		// try tar.gz, then zip, else return error
		if err := extractTarGz(src, dest); err == nil {
			return nil
		}
		if err := extractZip(src, dest); err == nil {
			return nil
		}
		return fmt.Errorf("unsupported archive format for %s", src)
	}
}

func actionChmod(path, modeStr string) error {
	if strings.TrimSpace(modeStr) == "" {
		return nil
	}
	return os.Chmod(path, parseMode(modeStr, 0o755))
}

func actionSymlink(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// remove existing symlink if existing
	if linksTo(dest, src) {
		return nil
	}
	_ = os.Remove(dest)
	return os.Symlink(src, dest)
}

func linksTo(link, want string) bool {
	got, err := os.Readlink(link)
	if err != nil {
		return false
	}
	return got == want
}

func parseMode(s string, fallback os.FileMode) os.FileMode {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return fallback
	}
	return os.FileMode(v)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("fileSHA256: open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("fileSHA256: read: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
