package main

import (
	"fmt"
	"log"

	"github.com/Imsol0/heimdall/pkg/runner"
)

func main() {
	fmt.Print(`
  _  _      _           _       _ _ 
 | || |___ (_)_ __  __| | __ _| | |
 | __ / -_)| | '  \/ _  |/ _  | | |
 |_||_\___||_|_|_|_\__,_|\__,_|_|_|
                                   
        CT Log Monitor - v3.0
        github.com/Imsol0/heimdall
`)

	options, err := runner.ParseOptions()
	if err != nil {
		log.Fatalf("Error parsing options: %v", err)
	}

	r, err := runner.NewRunner(options)
	if err != nil {
		log.Fatalf("Error creating runner: %v", err)
	}

	r.Run()
}
