package main

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/opencost/opencost/pkg/cmd"
)

func main() {
	fmt.Print("Starting")
	// runs the appropriate application mode using the default cost-model command
	// see: github.com/opencost/opencost/pkg/cmd package for details
	if err := cmd.Execute(nil); err != nil {
		log.Fatal().Err(err)
	}
}
