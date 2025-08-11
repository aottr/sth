package installer

import (
	"fmt"
	"strings"

	"github.com/aottr/sth/internal"
	"github.com/aottr/sth/internal/drivers/debian"
)

type Op string

const (
	OpEq   Op = "="
	OpGe   Op = ">="
	OpNone Op = "" // treat as exact upstream equality
)

type VersionConstraint struct {
	Op    Op
	Value string // user-specified version (may be upstream-only)
}

func ParseConstraint(s string) VersionConstraint {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, ">="):
		return VersionConstraint{Op: OpGe, Value: strings.TrimSpace(s[2:])}
	case strings.HasPrefix(s, "="):
		return VersionConstraint{Op: OpEq, Value: strings.TrimSpace(s[1:])}
	default:
		// bare value, treat as exact upstream equality
		return VersionConstraint{Op: OpNone, Value: s}
	}
}

func GetDriverForRelease(releaseID string, packages *internal.Packages) (Driver, error) {
	id := strings.ToLower(strings.TrimSpace(releaseID))

	debianFamily := map[string]struct{}{
		"debian":     {},
		"ubuntu":     {},
		"linuxmint":  {},
		"raspbian":   {},
		"pop":        {}, // Pop!_OS
		"neon":       {}, // KDE neon
		"kali":       {},
		"zorin":      {},
		"elementary": {},
	}

	// rhelFamily := map[string]struct{}{
	// 	"rhel":      {},
	// 	"rocky":     {},
	// 	"almalinux": {},
	// 	"centos":    {},
	// 	"fedora":    {},
	// 	"oracle":    {},
	// }

	if _, ok := debianFamily[id]; ok {
		if packages == nil {
			return debian.New(map[string]string{}), nil
		}
		return debian.New(packages.Apt), nil
	}
	// if _, ok := rhelFamily[id]; ok {
	// 	return RHELDriver{}
	// }

	// Default: try Debian semantics or return a no-op/unsupported driver
	return nil, fmt.Errorf("unsupported distro: %s", id)
}

type Driver interface {
	InstallAll() error
	Install([]string) error
}
