package internal

type Packages struct {
	Distro  string            `yaml:"distro"`
	Arch    string            `yaml:"arch"`
	Apt     map[string]string `yaml:"apt"`
	Flatpak map[string]string `yaml:"flatpak"`
	Recipes []string          `yaml:"recipes"`
}
type Dependencies struct {
	Apt     []string `yaml:"apt,omitempty"`
	Recipes []string `yaml:"recipes,omitempty"`
}

type RecipeReference struct {
	Name string `yaml:"name"`
}

type Recipe struct {
	Distro       string        `yaml:"distro"`
	Architecture string        `yaml:"arch"`
	Name         string        `yaml:"name"`
	Dependencies *Dependencies `yaml:"deps,omitempty"`
	Steps        []string      `yaml:"steps"`
}

type RecipeIndex struct {
	Recipes map[string]struct {
		Description string `yaml:"description"`
		Ubuntu      string `yaml:"ubuntu"`
	} `yaml:"recipes"`
}
