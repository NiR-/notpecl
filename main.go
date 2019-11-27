package main

import (
	"log"

	"github.com/NiR-/notpecl/cmd"
)

func main() {
	notpecl := cmd.NewRootCmd()

	if err := notpecl.Execute(); err != nil {
		log.Fatal(err)
	}
}
