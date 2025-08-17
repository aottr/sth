package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/aottr/sth/internal/brew"
)

type Spec struct {
	BrewFormulas []string
	AptPackages  []string
	Flatpaks     []string
}

func runFlatpak(ctx context.Context, refs []string) error {
	if len(refs) == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	args := append([]string{"install", "--noninteractive", "--assumeyes", "--or-update", "flathub"}, refs...)
	cmd := exec.CommandContext(ctx, "flatpak", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func runBrew(ctx context.Context, formulas []string) error {
	if len(formulas) == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	return brew.InstallWithBundle(ctx, formulas, brew.InstallOptions{
		Prefetch:     len(formulas) > 20,
		NoAutoUpdate: true,
	})
}

func InstallAll(spec Spec) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	tasks := []func() error{
		func() error { return runBrew(ctx, spec.BrewFormulas) },
		func() error { return runFlatpak(ctx, spec.Flatpaks) },
	}
	errs := make(chan error, len(tasks))

	submit := func(fn func() error) {
		select {
		case <-ctx.Done():
			errs <- ctx.Err()
		default:
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- fn()
			}()
		}
	}

	for _, t := range tasks {
		submit(t)
	}

	wg.Wait()
	close(errs)

	var failed []string
	for err := range errs {
		if err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("some installers failed:\n- %s", strings.Join(failed, "\n- "))
	}
	return nil
}
