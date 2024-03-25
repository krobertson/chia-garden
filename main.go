// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package main

import (
	"github.com/krobertson/chia-garden/cli"

	"go.uber.org/automaxprocs/maxprocs"

	_ "github.com/krobertson/chia-garden/cli/harvester"
	_ "github.com/krobertson/chia-garden/cli/plotter"
)

func main() {
	undo, _ := maxprocs.Set()
	defer undo()

	cli.Execute()
}
