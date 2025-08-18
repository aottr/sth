package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aottr/sth/internal/sthpkgs"
	"github.com/urfave/cli/v3"
)

func main() {

	cmd := &cli.Command{
		Name:  "sthpkgs",
		Usage: "Utility for sth recipe repositories",
		Commands: []*cli.Command{
			{
				Name:    "format",
				Aliases: []string{"f", "fmt"},
				Usage:   "format recipe files",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return nil
				},
			},
			{
				Name:    "generate",
				Aliases: []string{"g"},
				Usage:   "generate artifacts",
				Commands: []*cli.Command{
					{
						Name:  "index",
						Usage: "generate index.yaml",
						Action: func(ctx context.Context, cmd *cli.Command) error {

							if err := sthpkgs.GenerateIndex("recipes", "index.yaml"); err != nil {
								return err
							}
							fmt.Println("Wrote index.yaml")
							return nil
						},
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
