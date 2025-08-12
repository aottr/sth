package internal

import (
	"fmt"
	"os"

	"github.com/aottr/sth/internal/platform"
	"github.com/aottr/sth/internal/utils"
	"gopkg.in/yaml.v3"
)

type PackageType string

const (
	PackageTypeApt     PackageType = "apt"
	PackageTypeFlatpak PackageType = "flatpak"
	PackageTypeRecipe  PackageType = "recipes"
)

type Packages struct {
	path string

	Name     *string           `yaml:"name,omitempty"`
	Platform platform.Info     `yaml:"platform"`
	Apt      map[string]string `yaml:"apt"`
	Flatpak  map[string]string `yaml:"flatpak"`
	Recipes  []string          `yaml:"recipes"`
}

func LoadPackages(path string) (*Packages, error) {
	packages := &Packages{
		path: path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", path, err)
	}
	if err := yaml.Unmarshal(data, &packages); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}
	return packages, nil
}

func (p *Packages) Add(PackageType PackageType, pkgs []string) error {
	for _, pkg := range pkgs {
		p.addOne(PackageType, pkg)
	}
	if err := p.saveConfig(); err != nil {
		return err
	}
	return nil
}

func (p *Packages) addOne(PackageType PackageType, pkg string) {

	switch PackageType {
	case PackageTypeApt:
		p.Apt[pkg] = "latest"
	case PackageTypeFlatpak:
		p.Flatpak[pkg] = "latest"
	case PackageTypeRecipe:
		p.Recipes = append(p.Recipes, pkg)
	}
}

func (p *Packages) saveConfig() error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}
	if err := os.WriteFile(p.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write packages.yml: %v", err)
	}
	return nil
}

func Init(path, name string) (*Packages, error) {
	packages := &Packages{
		path:     path,
		Name:     utils.StringPtr(name),
		Platform: platform.GetPlatformInfo(),
	}
	if err := packages.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save packages.yml: %v", err)
	}
	return packages, nil
}
