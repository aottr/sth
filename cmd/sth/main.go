package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aottr/sth/internal"
	installer "github.com/aottr/sth/internal/drivers"
	"github.com/aottr/sth/internal/drivers/debian"
	"gopkg.in/yaml.v3"
)

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  sth install [file]      Install packages and recipes from YAML (default: packages.yml)")
	fmt.Println("  sth export [file]       Export installed packages to YAML (default: packages.yml)")
	fmt.Println("  sth list-recipes [url]  List available recipes from index URL")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
	}

	switch os.Args[1] {
	case "install":
		file := "packages.yml"
		if len(os.Args) > 2 {
			file = os.Args[2]
		}
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("❌ Failed to read %s: %v", file, err)
		}
		var pkgs internal.Packages
		if err := yaml.Unmarshal(data, &pkgs); err != nil {
			log.Fatalf("❌ Failed to parse YAML: %v", err)
		}

		// Install apt, flatpak packages
		deb := debian.New(pkgs.Apt)
		if err := deb.Install(); err != nil {
			log.Fatalf("❌ apt install failed: %v", err)
		}
		if err := installer.InstallFlatpak(pkgs.Flatpak); err != nil {
			log.Fatalf("❌ flatpak install failed: %v", err)
		}

		// Run remote recipes
		for _, recipeName := range pkgs.Recipes {
			recipe, err := installer.FetchRecipe(recipeName, pkgs.Distro)
			if err != nil {
				log.Fatalf("❌ Failed to fetch recipe '%s': %v", recipeName, err)
			}
			if err := installer.RunRecipe(recipeName, recipe); err != nil {
				log.Fatalf("❌ Recipe '%s' failed: %v", recipeName, err)
			}
		}

		fmt.Println("🎉 All installs and recipes completed successfully!")

	case "export":
		// Export currently installed packages (apt, snap, flatpak)
		outFile := "packages.yml"
		if len(os.Args) > 2 {
			outFile = os.Args[2]
		}
		// Using simplified export from previous example
		pkgs, _ := installer.ExportPackages()
		data, err := yaml.Marshal(&pkgs)
		if err != nil {
			log.Fatalf("❌ Failed to marshal YAML: %v", err)
		}
		if err := os.WriteFile(outFile, data, 0644); err != nil {
			log.Fatalf("❌ Failed to write %s: %v", outFile, err)
		}
		fmt.Printf("✅ Exported installed packages to %s\n", outFile)

	case "list-recipes":
		installer.ListRecipes()
	}
}
