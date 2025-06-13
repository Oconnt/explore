package main

import (
	"explore/cmd"
	"log"
	"os"
)

func main() {
	app := cmd.NewExp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
