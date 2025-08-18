package sthpkgs

import "strings"

type InstallScope string

const (
	InstallScopeUser   InstallScope = "user"
	InstallScopeSystem InstallScope = "system"
)

// RootDir:   "~/.local/sth"
// BinDir:    "<RootDir>/bin"
// PkgsDir:   "<RootDir>/pkgs"
// CacheDir:  "<RootDir>/cache"
// Manifests: "<RootDir>/manifests"
type Paths struct {
	RootDir   string `yaml:"rootDir,omitempty" json:"rootDir,omitempty"`
	BinDir    string `yaml:"binDir,omitempty" json:"binDir,omitempty"`
	PkgsDir   string `yaml:"pkgsDir,omitempty" json:"pkgsDir,omitempty"`
	CacheDir  string `yaml:"cacheDir,omitempty" json:"cacheDir,omitempty"`
	Manifests string `yaml:"manifests,omitempty" json:"manifests,omitempty"`
}

// Captured platform constraints to select the right package
type Target struct {
	OS     []string `yaml:"os,omitempty" json:"os,omitempty"`         // ["linux","darwin"]
	Distro string   `yaml:"distro,omitempty" json:"distro,omitempty"` // e.g., "ubuntu","debian"
	Family string   `yaml:"family,omitempty" json:"family,omitempty"` // e.g., "debian","rhel"
	Arch   []string `yaml:"arch,omitempty" json:"arch,omitempty"`     // ["amd64","arm64"]
}

type VersionSource struct {
	// "githubRelease", "githubTag", "httpJson", "regex", "static"
	Type     string `yaml:"type" json:"type"`
	Fallback string `yaml:"fallback,omitempty" json:"fallback,omitempty"`

	// For GitHub-based discovery
	Repo       string `yaml:"repo,omitempty" json:"repo,omitempty"`             // owner/name
	Prerelease bool   `yaml:"prerelease,omitempty" json:"prerelease,omitempty"` // include pre-releases
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"` // semver range, optional

	// For HTTP JSON discovery (tbd)
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	Selector string `yaml:"selector,omitempty" json:"selector,omitempty"` // JSONPath-like selector (e.g., "$.tag_name")

	// For regex scraping (fetch URL then apply regex with named group "version")
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`

	// For static versions
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

// Artifacts are downloadable items using templates... resolved after version has been discovered
// Template fields are Go text/template using a context that includes:
//
//	.Name, .Version, .OS, .Arch, .Distro, .Family etc
type Artifact struct {
	// Name defaults to recipe Name if empty
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Version discovery (latest-by-default).
	Version VersionSource `yaml:"version,omitempty" json:"version,omitempty"`

	// URL template for the asset to download
	URLTemplate string `yaml:"urlTemplate,omitempty" json:"urlTemplate,omitempty"`

	// Optional checksum template or URL to a checksum file
	SHA256Template string `yaml:"sha256Template,omitempty" json:"sha256Template,omitempty"`

	// Archive format: "raw","gz","tar.gz","zip"
	Format string `yaml:"format,omitempty" json:"format,omitempty"`

	// Path within archive to the binary to expose; for "raw" or single-file, leave empty
	InnerPath string `yaml:"innerPath,omitempty" json:"innerPath,omitempty"`

	// File mode for the exposed binary (octal string, 0755)
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

	// binary name in BinDir, defaults to Name
	BinName string `yaml:"binName,omitempty" json:"binName,omitempty"`
}

func (a Artifact) IsEmpty() bool {
	if strings.TrimSpace(a.URLTemplate) != "" {
		return false
	}
	if strings.TrimSpace(a.SHA256Template) != "" {
		return false
	}
	if strings.TrimSpace(a.InnerPath) != "" {
		return false
	}
	if strings.TrimSpace(a.Format) != "" {
		return false
	}
	if strings.TrimSpace(a.Name) != "" {
		return false
	}
	if strings.TrimSpace(a.BinName) != "" {
		return false
	}
	if strings.TrimSpace(a.Version.Type) != "" ||
		strings.TrimSpace(a.Version.Value) != "" ||
		strings.TrimSpace(a.Version.Fallback) != "" {
		return false
	}
	return true
}

func (a Artifact) GetFormatExtension() string {
	switch strings.ToLower(strings.TrimSpace(a.Format)) {
	case "gz":
		return ".gz"
	case "tar.gz", "tgz":
		return ".tar.gz"
	case "zip":
		return ".zip"
	default:
		return ""
	}
}

// InstallAction is a high-level action executed by the runner
// Default scope is user-space. set System: true for system actions (sudo)
// Args supports templating with context composed of .Paths, .ArtifactResolved, .Target, etc.
type InstallAction struct {
	Type   string            `yaml:"type" json:"type"`                     // "download","verify","extract","move","chmod","symlink","shell"
	Args   map[string]string `yaml:"args,omitempty" json:"args,omitempty"` // key-value args, template-capable
	System bool              `yaml:"system,omitempty" json:"system,omitempty"`
}

// ArtifactResolved is filled at runtime after version discovery and template rendering
type ArtifactResolved struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256,omitempty"`
	Format    string `json:"format"`
	InnerPath string `json:"innerPath,omitempty"`
	Mode      string `json:"mode,omitempty"`
	BinName   string `json:"binName,omitempty"`

	CacheFile  string `json:"cacheFile"`  // "<CacheDir>/<name>-<version>.<ext>"
	InstallDir string `json:"installDir"` // "<PkgsDir>/<name>-<version>"
	BinaryPath string `json:"binaryPath"` // "<InstallDir>/<innerPath or file>"
}

// Recipe is the main unit describing how to install a package
type Recipe struct {
	Name        string `yaml:"name" json:"name"`
	Slug        string `yaml:"slug,omitempty" json:"slug,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	Target Target       `yaml:"target,omitempty" json:"target,omitempty"`
	Scope  InstallScope `yaml:"scope,omitempty" json:"scope,omitempty"` // default: "user"

	Artifact Artifact        `yaml:"artifact,omitempty" json:"artifact,omitempty"`
	Actions  []InstallAction `yaml:"actions,omitempty" json:"actions,omitempty"`

	// to remove
	Steps []string `yaml:"steps,omitempty" json:"steps,omitempty"`

	Paths Paths `yaml:"paths,omitempty" json:"paths,omitempty"`
}

// RecipeIndex lists available recipes and helps discovery/UX.
type RecipeIndex struct {
	Recipes map[string]RecipeIndexEntry `yaml:"recipes" json:"recipes"`
}

type RecipeIndexEntry struct {
	Key         string `yaml:"-" json:"-"`
	Slug        string `yaml:"slug" json:"slug"`
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	Path   string   `yaml:"path" json:"path"`
	OS     []string `yaml:"os,omitempty" json:"os,omitempty"`
	Distro string   `yaml:"distro,omitempty" json:"distro,omitempty"`
	Family string   `yaml:"family,omitempty" json:"family,omitempty"`
	Arch   []string `yaml:"arch,omitempty" json:"arch,omitempty"`

	Scope InstallScope `yaml:"scope,omitempty" json:"scope,omitempty"`
}
