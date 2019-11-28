package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var gendocDestdir string

func NewGendocCmd(root *cobra.Command) *cobra.Command {
	gendoc := &cobra.Command{
		Use:    "gendoc",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			err := doc.GenMarkdownTree(root, gendocDestdir)
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	gendoc.Flags().StringVar(&gendocDestdir, "dest", "", "Destination directory")

	return gendoc
}
