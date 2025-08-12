package platform

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

const (
	FamilyDebian string = "debian"
	FamilyRHEL   string = "rhel"
	FamilyArch   string = "arch"
	FamilyOther  string = "other"
)

const DistroOther string = "other"

type Info struct {
	Arch   string `yaml:"arch"`             // e.g., "amd64"
	OS     string `yaml:"os"`               // e.g., "linux"
	Distro string `yaml:"distro,omitempty"` // distro ID, e.g., "ubuntu"
	Family string `yaml:"family,omitempty"` // e.g., "debian"
}

func GetPlatformInfo() Info {
	distro := DetectDistro()
	return Info{
		OS:     DetectOS(),
		Distro: distro,
		Family: DetectFamily(distro),
		Arch:   DetectArch(),
	}
}

// Normalize lowercases and trims
func Normalize(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

var debianIDs = set("debian", "ubuntu", "linuxmint", "raspbian", "pop", "neon", "kali", "zorin", "elementary")
var rhelIDs = set("rhel", "rocky", "almalinux", "centos", "fedora", "oracle")
var archIDs = set("arch", "manjaro", "endeavouros")

// generate a map from strings
func set(vals ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		m[v] = struct{}{}
	}
	return m
}

func DetectDistro() string {
	// try to find distro in /etc/os-release
	distro, found := findInFile("/etc/os-release", "ID=")
	if found {
		return distro
	}

	// try to find distro in /etc/lsb-release
	distro, found = findInFile("/etc/lsb-release", "DISTRIB_ID=")
	if found {
		return distro
	}
	return DistroOther
}

func DetectOS() string {
	return runtime.GOOS
}

func DetectArch() string {

	return runtime.GOARCH
}

func DetectFamily(id string) string {
	id = Normalize(string(id))
	if _, ok := debianIDs[id]; ok {
		return FamilyDebian
	}
	if _, ok := rhelIDs[id]; ok {
		return FamilyRHEL
	}
	if _, ok := archIDs[id]; ok {
		return FamilyArch
	}
	return FamilyOther
}

func findInFile(path string, needle string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, needle) {
			find := strings.TrimPrefix(line, needle)
			find = strings.Trim(find, "\"")
			return Normalize(find), true
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}
