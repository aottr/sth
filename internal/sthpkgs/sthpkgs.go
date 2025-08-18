package sthpkgs

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const RecipesBase = "https://raw.githubusercontent.com/aottr/sthpkgs/refs/heads/main/"

func FetchPackageRecipe(name string) (*Recipe, error) {
	resp, err := http.Get(RecipesBase + name)
	if err != nil {
		return nil, fmt.Errorf("failed to GET recipe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status %d when fetching recipe", resp.StatusCode)
	}
	var recipe Recipe
	dec := yaml.NewDecoder(resp.Body)
	if err := dec.Decode(&recipe); err != nil {
		return nil, fmt.Errorf("failed to parse recipe YAML: %w", err)
	}
	return &recipe, nil
}

func FetchRecipeIndex() (*RecipeIndex, error) {
	resp, err := http.Get(RecipesBase + "index.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to GET index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status %d when fetching index", resp.StatusCode)
	}
	var index RecipeIndex
	dec := yaml.NewDecoder(resp.Body)
	if err := dec.Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to parse index YAML: %w", err)
	}
	return &index, nil
}

func ListRecipes() error {

	idx, err := FetchRecipeIndex()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(idx.Recipes))
	for k := range idx.Recipes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e := idx.Recipes[k]
		var quals []string
		if len(e.OS) > 0 {
			quals = append(quals, "os="+strings.Join(e.OS, "|"))
		}
		if len(e.Arch) > 0 {
			quals = append(quals, "arch="+strings.Join(e.Arch, "|"))
		}
		if e.Scope != "" {
			quals = append(quals, "scope="+string(e.Scope))
		}
		fmt.Printf("%s: %s (%s) -> %s\n", k, e.Name, strings.Join(quals, ", "), e.Path)
	}
	return nil
}

// GenerateIndex scans recipesDir for */recipe.yaml and writes index.yaml to outPath.
func GenerateIndex(recipesDir, outPath string) error {
	files, err := scanRecipes(recipesDir)
	if err != nil {
		return fmt.Errorf("scan recipes: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no recipes found in %s", recipesDir)
	}

	idx := RecipeIndex{Recipes: make(map[string]RecipeIndexEntry)}
	for _, p := range files {
		r, err := loadRecipe(p)
		if err != nil {
			return fmt.Errorf("load %s: %w", p, err)
		}
		key := toIndexKeyWithFolder(r, p)
		if _, exists := idx.Recipes[key]; exists {
			return fmt.Errorf("duplicate index key %q (path %s)", key, p)
		}

		rel, _ := filepath.Rel(".", p)
		entry := RecipeIndexEntry{
			Slug:        r.Slug,
			Name:        r.Name,
			Description: r.Description,
			Path:        filepath.ToSlash(rel),
			OS:          cloneAndNormalizeList(r.Target.OS),
			Distro:      r.Target.Distro,
			Family:      r.Target.Family,
			Arch:        cloneAndNormalizeList(r.Target.Arch),
			Scope:       r.Scope,
		}
		entry.Key = key
		idx.Recipes[key] = entry
	}

	// Optional: stable ordering by key for nicer diffs
	ordered := orderIndex(idx)

	out, err := yaml.Marshal(&ordered)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func scanRecipes(recipesDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(recipesDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if name == "recipe.yml" || name == "recipe.yaml" {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

func loadRecipe(path string) (Recipe, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, err
	}
	var r Recipe
	if err := yaml.Unmarshal(b, &r); err != nil {
		return Recipe{}, err
	}

	// Basic validation/defaults
	if strings.TrimSpace(r.Slug) == "" {
		r.Slug = filepath.Base(filepath.Dir(path))
	}
	if strings.TrimSpace(r.Name) == "" {
		r.Name = r.Slug
	}
	if r.Scope == "" {
		r.Scope = InstallScopeUser
	}
	// Normalize OS/Arch slices (lowercase)
	r.Target.OS = cloneAndNormalizeList(r.Target.OS)
	r.Target.Arch = cloneAndNormalizeList(r.Target.Arch)

	return r, nil
}

// toIndexKey builds a stable key for the recipe in the index.
// Strategy:
//   - base: slug
//   - qualifiers: if Target.OS or Target.Arch restricts to one or more values,
//     include them; also include "system" when scope == system
//   - examples:
//     kubectl                      (no OS/Arch restriction, user scope)
//     kubectl@linux,darwin        (restricted OSes)
//     kubectl@amd64,arm64         (restricted arches)
//     kubectl@linux-amd64         (single os and arch)
//     kubectl@system              (system scope)
//     kubectl@linux,arm64,system  (combined)
func toIndexKey(r Recipe) string {
	base := r.Slug
	var quals []string

	osList := cloneAndNormalizeList(r.Target.OS)
	archList := cloneAndNormalizeList(r.Target.Arch)

	switch {
	case len(osList) == 1 && len(archList) == 1:
		quals = append(quals, fmt.Sprintf("%s-%s", osList[0], archList[0]))
	case len(osList) == 1 && len(archList) == 0:
		quals = append(quals, osList[0])
	case len(osList) == 0 && len(archList) == 1:
		quals = append(quals, archList[0])
	default:
		// For multi-value lists, add them as comma-separated groups for visibility
		// We keep them as separate qualifiers to avoid ambiguity
		if len(osList) > 1 {
			quals = append(quals, strings.Join(osList, "|"))
		} else if len(osList) == 1 {
			quals = append(quals, osList[0])
		}
		if len(archList) > 1 {
			quals = append(quals, strings.Join(archList, "|"))
		} else if len(archList) == 1 {
			quals = append(quals, archList[0])
		}
	}

	if r.Scope == InstallScopeSystem {
		quals = append(quals, "system")
	}

	if len(quals) == 0 {
		return base
	}
	return base + "@" + strings.Join(quals, ",")
}

func toIndexKeyWithFolder(r Recipe, recipePath string) string {
	// "recipes/nvim.appimage/recipe.yml" -> "nvim.appimage"
	folder := filepath.Base(filepath.Dir(recipePath))
	base := folder
	// If folder is empty for any reason, fall back to slug
	if strings.TrimSpace(base) == "" {
		base = strings.TrimSpace(r.Slug)
		if base == "" {
			base = "unknown"
		}
	}

	osList := cloneAndNormalizeList(r.Target.OS)
	archList := cloneAndNormalizeList(r.Target.Arch)

	key := base
	switch {
	case len(osList) == 1 && len(archList) == 1:
		key = fmt.Sprintf("%s@%s-%s", base, osList[0], archList[0])
	case len(osList) == 1 && len(archList) == 0:
		key = fmt.Sprintf("%s@%s", base, osList[0])
	case len(osList) == 0 && len(archList) == 1:
		key = fmt.Sprintf("%s@%s", base, archList[0])
	}

	if r.Scope == InstallScopeSystem {
		if strings.Contains(key, "@") {
			key += ",system"
		} else {
			key += "@system"
		}
	}
	return key
}

// removing empty entries and duplicates, and sorting ascending
func cloneAndNormalizeList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, v := range in {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// orderIndex returns copy of the index ordered by key
func orderIndex(idx RecipeIndex) RecipeIndex {
	keys := make([]string, 0, len(idx.Recipes))
	for k := range idx.Recipes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := RecipeIndex{Recipes: make(map[string]RecipeIndexEntry, len(idx.Recipes))}
	for _, k := range keys {
		out.Recipes[k] = idx.Recipes[k]
	}
	return out
}
