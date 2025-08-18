package utils

import (
	"strconv"
	"strings"
)

func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func WithDefault(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

type SemVer struct{ major, minor, patch int }

func ParseSemVer(v string) (SemVer, bool) {
	// expects 1.2.3 (no leading v)
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return SemVer{}, false
	}
	mv, err1 := strconv.Atoi(parts[0])
	nv, err2 := strconv.Atoi(parts[1])
	pv, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return SemVer{}, false
	}
	return SemVer{mv, nv, pv}, true
}
func CmpSemVer(a, b SemVer) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return 0
}
