// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package main

import (
	"github.com/krobertson/chia-garden/cli"

	_ "github.com/krobertson/chia-garden/cli/harvester"
	_ "github.com/krobertson/chia-garden/cli/plotter"
)

func main() {
	cli.Execute()
}
