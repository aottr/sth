package utils

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

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
	return "linux"
}

func DetectArch() string {

	return runtime.GOARCH
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
			find = strings.TrimSpace(find)
			return strings.ToLower(find), true
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}
