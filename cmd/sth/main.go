package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aottr/sth/internal"
	installer "github.com/aottr/sth/internal/drivers"
	"github.com/aottr/sth/internal/drivers/debian"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func main() {

	cmd := &cli.Command{
		Name:  "sth",
		Usage: "Just install sth",
		Commands: []*cli.Command{
			{
				Name:    "install",
				Aliases: []string{"i"},
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:  "file",
						Value: "packages.yml",
					},
				},
				Usage: "Install packages and recipes from YAML",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					data, err := os.ReadFile(cmd.StringArg("file"))
					if err != nil {
						log.Fatalf("❌ Failed to read %s: %v", cmd.StringArg("file"), err)
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
						// check if installed
						if installer.IsInstalled(recipeName) {
							fmt.Println("🔄 Skipping already installed recipe package: ", recipeName)
							continue
						}
						if err := installer.RunRecipe(recipeName, recipe); err != nil {
							log.Fatalf("❌ Recipe '%s' failed: %v", recipeName, err)
						}
					}

					fmt.Println("🎉 All installs and recipes completed successfully!")
					return nil
				},
			},
			{
				Name:    "recipe",
				Aliases: []string{"r", "res"},
				Usage:   "Run or find a recipe",
				Commands: []*cli.Command{
					{
						Name:    "list",
						Aliases: []string{"l", "ls"},
						Usage:   "List all recipes",
						Action: func(ctx context.Context, c *cli.Command) error {
							installer.ListRecipes()
							return nil
						},
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}

	// switch os.Args[1] {
	// case "export":
	// 	// Export currently installed packages (apt, snap, flatpak)
	// 	outFile := "packages.yml"
	// 	if len(os.Args) > 2 {
	// 		outFile = os.Args[2]
	// 	}
	// 	// Using simplified export from previous example
	// 	pkgs, _ := installer.ExportPackages()
	// 	data, err := yaml.Marshal(&pkgs)
	// 	if err != nil {
	// 		log.Fatalf("❌ Failed to marshal YAML: %v", err)
	// 	}
	// 	if err := os.WriteFile(outFile, data, 0644); err != nil {
	// 		log.Fatalf("❌ Failed to write %s: %v", outFile, err)
	// 	}
	// 	fmt.Printf("✅ Exported installed packages to %s\n", outFile)

}
