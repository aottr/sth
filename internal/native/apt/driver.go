package apt

import (
	"fmt"
	"os/exec"
	"strings"

	// installer "github.com/aottr/sth/internal/drivers"
	"github.com/aottr/sth/internal/utils"
)

type DebianDriver struct {
	Packages map[string]string
}

func New(packages map[string]string) *DebianDriver {
	return &DebianDriver{
		Packages: packages,
	}
}

func (d *DebianDriver) InstallAll() error {
	if len(d.Packages) == 0 {
		return nil
	}
	fmt.Println("🔄 Running apt update")
	if _, err := utils.RunCommand("sudo", "apt", "update"); err != nil {
		return err
	}
	for pkg := range d.Packages {

		if err := ensureLatest(pkg); err != nil {
			return err
		}
		// c := installer.ParseConstraint(version)
		// switch {
		// case strings.EqualFold(version, "latest"):
		// 	if err := ensureLatest(pkg); err != nil {
		// 		return err
		// 	}
		// default:
		// figure out how to get apt versions to follow upstream...
		// if err := ensureLatest(pkg); err != nil {
		// 	return err
		// }
		// if err := ensureVersion(pkg, c); err != nil {
		// 	return err
		// }
		//}
	}
	return nil
}

func (d *DebianDriver) Install(pkgs []string) error {
	if _, err := utils.RunCommand("sudo", "apt", "update"); err != nil {
		return err
	}
	for _, pkg := range pkgs {
		if err := ensureLatest(pkg); err != nil {
			return err
		}
	}
	return nil
}

func ensureLatest(pkg string) error {
	if IsInstalled(pkg) {
		fmt.Println("🔄 Skipping already installed apt package: ", pkg)
		return nil
	}
	fmt.Printf("📦 Installing latest apt package: %s\n", pkg)
	if _, err := utils.RunCommand("sudo", "apt", "install", "-y", pkg); err != nil {
		return err
	}
	return nil
}

// func ensureVersion(pkg string, c installer.VersionConstraint) error {
// 	// chek if need to install
// 	ver, err := getInstalledVersion(pkg)
// 	if err != nil {
// 		return err
// 	}
// 	installed, err := satisfiesConstraint(ver, c)
// 	if err != nil {
// 		return err
// 	}
// 	if installed {
// 		fmt.Printf("🔄 Skipping apt package %s; installed %q satisfies %s %s\n", pkg, ver, string(c.Op), c.Value)
// 		return nil
// 	}

// 	// installing requested version
// 	fmt.Printf("📦 Installing/Upgrading apt package to satisfy %s %s: %s\n", string(c.Op), c.Value, pkg)
// 	if _, err := utils.RunCommand("sudo", "apt", "install", "-y", pkg); err != nil {
// 		return err
// 	}
// 	// Re-check
// 	ver, _ = getInstalledVersion(pkg)
// 	ok, err := satisfiesConstraint(ver, c)
// 	if err != nil {
// 		return err
// 	}
// 	if !ok {
// 		return fmt.Errorf("after install, %s version %q does not satisfy %s %s", pkg, ver, string(c.Op), c.Value)
// 	}

// 	// mark package as held if version is pinned
// 	if c.Op != installer.OpGe {
// 		fmt.Printf("⛔ Holding apt package: %s\n", pkg)
// 		if _, err := utils.RunCommand("sudo", "apt-mark", "hold", pkg); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

func IsInstalled(pkg string) bool {
	cmd := exec.Command("dpkg", "-s", pkg)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

//// VERSION CHECKS ////

// getInstalledVersion returns the full Debian version string or empty if not installed.
func getInstalledVersion(pkg string) (string, error) {
	cmd := exec.Command("dpkg-query", "-W", "-f", "${Version}\n", pkg)
	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			// package not installed
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseDpkgVersion splits a Debian version into epoch, upstream, debianRev.
// Examples:
//
//	"1:2.43.0-1ubuntu7.3" -> ("1", "2.43.0", "1ubuntu7.3")
//	"2.43.0-1"            -> ("",  "2.43.0", "1")
//	"2.43.0"              -> ("",  "2.43.0", "")
func parseDpkgVersion(v string) (epoch, upstream, debRev string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", ""
	}
	// epoch
	if i := strings.IndexByte(v, ':'); i != -1 {
		epoch = v[:i]
		v = v[i+1:]
	}
	// debian revision
	if j := strings.LastIndexByte(v, '-'); j != -1 {
		upstream = v[:j]
		debRev = v[j+1:]
	} else {
		upstream = v
	}
	return
}

// dpkgCompare uses Debian comparator. Returns true if "a <op> b" (per dpkg --compare-versions).
func dpkgCompare(a, op, b string) (bool, error) {
	cmd := exec.Command("dpkg", "--compare-versions", a, op, b)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// Non-zero exit means comparison false, but also could be usage error if inputs empty
	if ee, ok := err.(*exec.ExitError); ok {
		if ee.ExitCode() == 1 {
			return false, nil
		}
	}
	return false, err
}

// extractUpstream returns only the upstream component from a Debian version.
func extractUpstream(v string) string {
	_, upstream, _ := parseDpkgVersion(v)
	return upstream
}

// func satisfiesConstraint(installedVersion string, c installer.VersionConstraint) (bool, error) {
// 	if installedVersion == "" {
// 		return false, nil
// 	}
// 	wantedVersion := strings.TrimSpace(c.Value)
// 	if wantedVersion == "" {
// 		return false, nil
// 	}
// 	upstreamVersion := extractUpstream(installedVersion)
// 	switch c.Op {
// 	case installer.OpEq, installer.OpNone:
// 		return dpkgCompare(upstreamVersion, "eq", wantedVersion)
// 	case installer.OpGe:
// 		return dpkgCompare(upstreamVersion, "ge", wantedVersion)
// 	default:
// 		return false, fmt.Errorf("unsupported operator: %q", c.Op)
// 	}
// }
