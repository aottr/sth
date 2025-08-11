package native

import (
	"bufio"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExportPackages runs "apt list --installed" and returns a slice of package names
func ExportPackages() ([]string, error) {
	cmd := exec.Command("apt", "list", "--installed")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var packages []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		// skip header or malformed lines
		if strings.HasPrefix(line, "Listing") {
			continue
		}
		// lines look like: package/version [architecture] [installed]
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			pkgName := parts[0]
			packages = append(packages, pkgName)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return packages, nil
}

func ExportPackagesToYAML(packages []string, filepath string) error {
	data, err := yaml.Marshal(packages)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}
