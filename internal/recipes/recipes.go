package recipes

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/aottr/sth/internal"
	"github.com/aottr/sth/internal/cache"
	"github.com/aottr/sth/internal/utils"
	"gopkg.in/yaml.v3"
)

const RecipeIndex = "https://raw.githubusercontent.com/aottr/sthpkgs/refs/heads/main/index.yml"
const RecipesBase = "https://raw.githubusercontent.com/aottr/sthpkgs/refs/heads/main/"

func FetchRecipe(entry internal.RecipeIndexEntry) (*internal.Recipe, error) {
	fmt.Printf("🌐 Downloading recipe for %s\n", entry.Name)
	resp, err := http.Get(RecipesBase + entry.Path)
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

func fetchRecipeIndex(url string) (*internal.RecipeIndex, error) {
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
	cache.SaveCache(".sth.cache", &index)
	return &index, nil
}

func GetRecipeIndex(url string) (*internal.RecipeIndex, error) {
	var index internal.RecipeIndex

	var err error
	fresh := cache.CacheIsFresh(".sth.cache", time.Hour*12)
	if fresh {
		fmt.Println("Using cached recipe index")
		err = cache.LoadCache(".sth.cache", &index)
		if err != nil {
			fmt.Println("failed to load cache. Continuing with remote fetch.")
		}
	}
	if !fresh || err != nil {
		index, err := fetchRecipeIndex(url)
		if err != nil {
			return nil, err
		}
		return index, nil

	}
	return &index, nil
}

func FindRecipe(name string) (*internal.RecipeIndexEntry, error) {
	index, err := GetRecipeIndex(RecipeIndex)
	if err != nil {
		return nil, err
	}
	for _, recipe := range index.Recipes {
		if strings.Contains(recipe.Slug, name) {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

// ListRecipes fetches and lists recipe names and descriptions from a remote index URL
func ListRecipes() error {

	index, err := GetRecipeIndex(RecipeIndex)
	if err != nil {
		return err
	}

	fmt.Println("Available recipes:")
	for name, meta := range index.Recipes {
		fmt.Printf(" - %s: %s\n", name, meta.Description)
	}
	return nil
}
