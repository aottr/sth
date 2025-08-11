package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aottr/sth/internal"
	"github.com/aottr/sth/internal/flatpak"
	"github.com/aottr/sth/internal/native"
	"github.com/aottr/sth/internal/recipes"
	"github.com/urfave/cli/v3"
)

func main() {

	cmd := &cli.Command{
		Name:  "sth",
		Usage: "Just install sth",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Usage:   "packages config file",
				Value:   "packages.yml",
				Aliases: []string{"f"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "install",
				Aliases: []string{"i"},
				Usage:   "Install packages and recipes from YAML",
				Action: func(ctx context.Context, cmd *cli.Command) error {

					pkgs, err := internal.LoadPackages(cmd.String("file"))
					if err != nil {
						log.Fatalf("❌ Failed to load packages: %v", err)
					}

					// Install apt, flatpak packages
					driver, err := native.GetDriverForRelease(pkgs.Distro, pkgs)
					if err != nil {
						log.Fatalf("❌ Failed to get driver for distro: %v", err)
					}
					if err := driver.InstallAll(); err != nil {
						log.Fatalf("❌ apt install failed: %v", err)
					}
					if err := flatpak.InstallFlatpak(pkgs.Flatpak); err != nil {
						log.Fatalf("❌ flatpak install failed: %v", err)
					}

					// Run remote recipes
					for _, recipeName := range pkgs.Recipes {
						recipe, err := recipes.FetchRecipe(recipeName, pkgs.Distro)
						if err != nil {
							log.Fatalf("❌ Failed to fetch recipe '%s': %v", recipeName, err)
						}
						// check if installed
						if recipes.IsInstalled(recipeName) {
							fmt.Println("🔄 Skipping already installed recipe package: ", recipeName)
							continue
						}
						if err := recipes.RunRecipe(recipeName, recipe); err != nil {
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
							recipes.ListRecipes()
							return nil
						},
					},
				},
			},
			{
				Name:    "add",
				Aliases: []string{"a"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "type",
						Usage:   "[apt|flatpak|recipe]",
						Value:   "apt",
						Aliases: []string{"t"},
						Validator: func(t string) error {
							if t == "apt" || t == "flatpak" || t == "recipe" {
								return nil
							}
							return fmt.Errorf("invalid package type: %s", t)
						},
					},
				},
				Arguments: []cli.Argument{
					&cli.StringArgs{
						Name: "package",
						Min:  0,
						Max:  10,
					},
				},
				Usage: "Add a package to the list",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					pkgs, err := internal.LoadPackages(cmd.String("file"))
					if err != nil {
						log.Fatalf("❌ Failed to load packages: %v", err)
					}
					pkg := cmd.StringArgs("package")
					if len(pkg) == 0 {
						return fmt.Errorf("no package specified")
					}
					err = pkgs.Add(internal.PackageTypeApt, pkg)
					if err != nil {
						return err
					}
					fmt.Println("✅ Added package to list")
					driver, err := native.GetDriverForRelease(pkgs.Distro, nil)
					if err != nil {
						log.Fatalf("❌ Failed to get driver for distro: %v", err)
					}
					err = driver.Install(pkg)
					return err
				},
			},
			{
				Name:  "init",
				Usage: "Initialize packages.yml",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					_, err := internal.Init(cmd.String("file"))
					if err != nil {
						log.Fatalf("❌ Failed to initialize packages: %v", err)
					}
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
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
