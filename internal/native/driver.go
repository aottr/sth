package native

import (
	"fmt"
	"strings"

	"github.com/aottr/sth/internal"
	"github.com/aottr/sth/internal/native/apt"
	"github.com/aottr/sth/internal/platform"
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

func GetDriverForRelease(family string, packages *internal.Packages) (Driver, error) {

	switch family {
	case platform.FamilyDebian:
		if packages == nil {
			return apt.New(map[string]string{}), nil
		}
		return apt.New(packages.Apt), nil
	}

	return nil, fmt.Errorf("unsupported system: %s", family)
}

type Driver interface {
	InstallAll() error
	Install([]string) error
}
