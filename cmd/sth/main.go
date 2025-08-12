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
						log.Fatalf("failed to load packages: %v", err)
					}

					// Install apt, flatpak packages
					driver, err := native.GetDriverForRelease(pkgs.Platform.Family, pkgs)
					if err != nil {
						log.Fatalf("failed to get driver for distro: %v", err)
					}
					if err := driver.InstallAll(); err != nil {
						log.Fatalf("apt install failed: %v", err)
					}
					if err := flatpak.InstallFlatpak(pkgs.Flatpak); err != nil {
						log.Fatalf("flatpak install failed: %v", err)
					}

					// Run remote recipes
					for _, recipeName := range pkgs.Recipes {
						rr, err := recipes.FindRecipe(recipeName)
						if err != nil {
							return fmt.Errorf("recipe not found")
						}
						recipe, err := recipes.FetchRecipe(*rr)
						if err != nil {
							log.Fatalf("failed to fetch recipe '%s': %v", recipeName, err)
						}
						// check if installed
						if recipes.IsInstalled(recipeName) {
							fmt.Println("🔄 Skipping already installed recipe package: ", recipeName)
							continue
						}
						if err := recipes.RunRecipe(recipeName, recipe); err != nil {
							log.Fatalf("recipe '%s' failed: %v", recipeName, err)
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
					{
						Name:    "find",
						Aliases: []string{"f"},
						Usage:   "Find a recipe by name",
						Action: func(ctx context.Context, c *cli.Command) error {
							slug, err := recipes.FindRecipe(c.Args().First())
							if err != nil {
								return err
							}
							fmt.Println(*slug)
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
							switch t {
							case "apt", "flatpak", "recipe":
								return nil
							default:
								return fmt.Errorf("invalid package type: %s", t)
							}
						},
					},
				},
				Arguments: []cli.Argument{
					&cli.StringArgs{
						Name: "package",
						Min:  0,
						Max:  20,
					},
				},
				Usage: "Add a package to the list",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					pkgConfig, err := internal.LoadPackages(cmd.String("file"))
					if err != nil {
						log.Fatalf("failed to load packages: %v", err)
					}

					packageType := cmd.String("type")
					names := cmd.StringArgs("package")
					if len(names) == 0 {
						return fmt.Errorf("no package specified")
					}

					switch packageType {
					case "apt":
						if err := pkgConfig.Add(internal.PackageTypeApt, names); err != nil {
							return err
						}
						driver, err := native.GetDriverForRelease(pkgConfig.Platform.Family, nil)
						if err != nil {
							log.Fatalf("failed to get driver for distro: %v", err)
						}
						err = driver.Install(names)
						if err != nil {
							return err
						}
					case "flatpak":
						if err := pkgConfig.Add(internal.PackageTypeFlatpak, names); err != nil {
							return err
						}
						if err := flatpak.InstallFlatpak(pkgConfig.Flatpak); err != nil {
							log.Fatalf("flatpak install failed: %v", err)
						}
					case "recipe":
						if err := pkgConfig.Add(internal.PackageTypeRecipe, names); err != nil {
							return err
						}
						for _, recipeName := range names {
							rr, err := recipes.FindRecipe(recipeName)
							if err != nil {
								return fmt.Errorf("recipe not found")
							}
							recipe, err := recipes.FetchRecipe(*rr)
							if err != nil {
								log.Fatalf("failed to fetch recipe '%s': %v", recipeName, err)
							}
							// check if installed
							if recipes.IsInstalled(recipeName) {
								fmt.Println("🔄 Skipping already installed recipe package: ", recipeName)
								continue
							}
							if err := recipes.RunRecipe(recipeName, recipe); err != nil {
								log.Fatalf("recipe '%s' failed: %v", recipeName, err)
							}
						}
					}
					return nil
				},
			},
			{
				Name:  "init",
				Usage: "Initialize packages.yml",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:  "name",
						Value: "New System",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					_, err := internal.Init(cmd.String("file"), cmd.StringArg("name"))
					if err != nil {
						log.Fatalf("failed to initialize packages: %v", err)
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
}
