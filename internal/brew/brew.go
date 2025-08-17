package brew

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type InstallOptions struct {
	Prefetch     bool
	NoAutoUpdate bool
}

const (
	SmallInstallationThreshold = 3
	PrefetchWorkers            = 3
)

// normalizeFormula makes a formula comparable to `brew list --formula` output
// converts "tap/name/name" or "tap/name" to "name"
func normalizeFormula(n string) string {
	n = strings.TrimSpace(n)
	if n == "" {
		return n
	}
	parts := strings.Split(n, "/")
	return parts[len(parts)-1]
}

// FilterInstalled returns only formulas that are not currently installed
func FilterInstalled(ctx context.Context, names []string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "brew", "list", "--formula")
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("brew list failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	installed := make(map[string]struct{}, len(lines)) // lets be memory efficient xD
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			installed[line] = struct{}{}
		}
	}
	var toInstall []string
	for _, raw := range names {
		if raw = strings.TrimSpace(raw); raw == "" {
			continue
		}
		short := normalizeFormula(raw)
		if _, ok := installed[short]; !ok {
			toInstall = append(toInstall, raw)
		}
	}
	return toInstall, nil
}

// cachedBottleExists returns true if brew reports a cache path that exists
func cachedBottleExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "brew", "--cache", name)
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	if err == nil {
		if fi.Mode().IsRegular() {
			return true
		}
		if fi.IsDir() {
			ents, _ := os.ReadDir(path)
			return len(ents) > 0
		}
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	// Some cache path outputs may represent a pattern...
	matches, _ := filepath.Glob(filepath.Join(dir, base+"*"))
	return len(matches) > 0
}

// Prefetch runs brew fetch concurrently for only uncached formulas
func Prefetch(ctx context.Context, packages []string, workers int) {
	if workers <= 0 {
		workers = PrefetchWorkers
	}
	numCPU := runtime.NumCPU()
	if workers > numCPU {
		workers = numCPU
	}
	// Build queue of uncached only
	queue := make([]string, 0, len(packages))
	for _, p := range packages {
		select {
		case <-ctx.Done():
			fmt.Println("[brew] prefetch canceled before queue build finished")
			return
		default:
		}
		if !cachedBottleExists(ctx, p) {
			queue = append(queue, p)
		}
	}
	if len(queue) == 0 {
		fmt.Println("[brew] prefetch skipped: all bottles cached")
		return
	}

	fmt.Printf("[brew] prefetch queue: %d formulas (workers=%d)\n", len(queue), workers)

	jobs := make(chan string)
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for n := range jobs {
			cmd := exec.CommandContext(ctx, "brew", "fetch", "--retry", n)
			cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run() // best-effort, will be fetched anyway if necessary
		}
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}
	for _, n := range queue {
		select {
		case jobs <- n:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
	fmt.Printf("[brew] prefetch completed\n")
}

// writeBrewfile creates a minimal Brewfile containing only "brew" lines
func writeBrewfile(path string, formulas []string) error {
	var b strings.Builder
	for _, n := range formulas {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		// b.WriteString(fmt.Sprintf("tap \"homebrew/core\"\n"))
		b.WriteString(fmt.Sprintf("brew \"%s\"\n", n))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// InstallWithBundle generates a Brewfile and calls `brew bundle install`
func InstallWithBundle(ctx context.Context, packages []string, opts InstallOptions) error {
	// Default NoAutoUpdate true
	if !opts.NoAutoUpdate {
		opts.NoAutoUpdate = true
	}

	fmt.Printf("[brew] resolving %d requested formulas...\n", len(packages))
	toInstall, err := FilterInstalled(ctx, packages)
	if err != nil {
		return err
	}
	if len(toInstall) == 0 {
		fmt.Println("[brew] all requested formulas are already installed")
		return nil
	}
	fmt.Printf("[brew] %d formulas need installation\n", len(toInstall))

	if opts.Prefetch && !(len(toInstall) < SmallInstallationThreshold) {
		fmt.Printf("[brew] prefetching up to %d formulas with %d workers...\n", len(toInstall), PrefetchWorkers)
		ctxFetch, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		Prefetch(ctxFetch, toInstall, PrefetchWorkers)
		fmt.Println("[brew] prefetch phase completed")
	}

	// gen temp Brewfile
	tmpDir := os.TempDir()
	brewfile := filepath.Join(tmpDir, "Brewfile.sth")
	if err := writeBrewfile(brewfile, toInstall); err != nil {
		return fmt.Errorf("failed to write Brewfile: %w", err)
	}

	args := []string{"bundle", "install", "--quiet", "--file", brewfile}
	cmd := exec.CommandContext(ctx, "brew", args...)

	env := append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1", "HOMEBREW_NO_INSTALL_CLEANUP=1", "HOMEBREW_INSTALL_BADGE=🦦")
	if opts.NoAutoUpdate {
		env = append(env, "HOMEBREW_NO_AUTO_UPDATE=1")
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew bundle install failed: %w", err)
	}
	fmt.Println("[brew] bundle install completed")
	return nil
}
