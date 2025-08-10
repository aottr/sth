package installer

import (
	"fmt"

	"github.com/aottr/sth/internal/utils"
)

func InstallFlatpak(pkgs map[string]string) error {
	if len(pkgs) == 0 {
		return nil
	}
	for pkg := range pkgs {
		fmt.Printf("📦 Installing flatpak: %s\n", pkg)
		if _, err := utils.RunCommand("flatpak", "install", "-y", pkg); err != nil {
			return err
		}
	}
	return nil
}
