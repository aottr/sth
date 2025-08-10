package installer

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/aottr/sth/internal"
	"github.com/aottr/sth/internal/utils"
	"gopkg.in/yaml.v3"
)

const RecipeIndex = "https://raw.githubusercontent.com/aottr/sthpkgs/refs/heads/main/index.yml"
const RecipesBase = "https://raw.githubusercontent.com/aottr/sthpkgs/refs/heads/main/"

func FetchRecipe(name string, distro string) (*internal.Recipe, error) {
	fmt.Printf("🌐 Downloading recipe for %s\n", name)
	resp, err := http.Get(RecipesBase + name + "/" + distro + ".yml")
	if err != nil {
		return nil, fmt.Errorf("failed to GET recipe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status %d when fetching recipe", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipe body: %w", err)
	}
	var recipe internal.Recipe
	if err := yaml.Unmarshal(body, &recipe); err != nil {
		return nil, fmt.Errorf("failed to parse recipe YAML: %w", err)
	}
	return &recipe, nil
}

func RunRecipe(name string, recipe *internal.Recipe) error {
	fmt.Printf("🚀 Running recipe: %s (%d steps)\n", name, len(recipe.Steps))
	for i, step := range recipe.Steps {
		fmt.Printf("[%s] Step %d/%d: %s\n", name, i+1, len(recipe.Steps), step)
		if err := utils.RunBashCommand(step, false); err != nil {
			return fmt.Errorf("[%s] step %d failed: %w", name, i+1, err)
		}
	}
	fmt.Printf("✅ Recipe %s completed!\n", name)
	return nil
}

func IsInstalled(pkg string) bool {
	_, err := exec.LookPath(pkg)
	return err == nil
}

func FetchRecipeIndex(url string) (*internal.RecipeIndex, error) {
	fmt.Printf("🌐 Downloading recipe index from %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to GET recipe index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status %d when fetching recipe index", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipe index body: %w", err)
	}
	var index internal.RecipeIndex
	if err := yaml.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse recipe index YAML: %w", err)
	}
	return &index, nil
}

// ListRecipes fetches and lists recipe names and descriptions from a remote index URL
func ListRecipes() error {
	resp, err := http.Get(RecipeIndex)
	if err != nil {
		return fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read index response: %w", err)
	}

	var index internal.RecipeIndex
	err = yaml.Unmarshal(data, &index)
	if err != nil {
		return fmt.Errorf("failed to parse index YAML: %w", err)
	}

	fmt.Println("Available recipes:")
	for name, meta := range index.Recipes {
		fmt.Printf(" - %s: %s\n", name, meta.Description)
	}
	return nil
}
