package installer

import "strings"

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

type Driver interface {
	Install(pkg string) error
}
