package internal

type Dependencies struct {
	// Native, family-specific channels
	Apt []string `yaml:"apt,omitempty"`
	// Cross-recipe references
	Recipes []string `yaml:"recipes,omitempty"`
}

type Target struct {
	OS     string `yaml:"os,omitempty"`     // e.g., "linux"
	Distro string `yaml:"distro,omitempty"` // e.g., ["ubuntu","debian"]
	Family string `yaml:"family,omitempty"` // e.g., ["debian","rhel"]
	Arch   string `yaml:"arch,omitempty"`   // e.g., ["amd64","arm64"]
}

type RecipeReference struct {
	Name string `yaml:"name"`
}

type Recipe struct {
	Name         string       `yaml:"name"`
	Target       Target       `yaml:"target,omitempty"`
	Dependencies Dependencies `yaml:"deps,omitempty"`
	Steps        []string     `yaml:"steps"`
}

type RecipeIndex struct {
	Recipes map[string]RecipeIndexEntry `yaml:"recipes"`
}

type RecipeIndexEntry struct {
	Key         string `yaml:"-"` // filled during load from map key
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`

	Path   string       `yaml:"path"`
	OS     string       `yaml:"os,omitempty"`     // e.g., ["linux","drawin"]
	Distro string       `yaml:"distro,omitempty"` // e.g., ["ubuntu","debian"]
	Family string       `yaml:"family,omitempty"` // e.g., ["debian","rhel"]
	Arch   string       `yaml:"arch,omitempty"`   // e.g., ["amd64","arm64"]
	Deps   Dependencies `yaml:"deps,omitempty"`
}
