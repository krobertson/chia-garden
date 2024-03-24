package rpc

import (
	"github.com/krobertson/chia-garden/pkg/types"
)

type Harvester interface {
	PlotReady(*types.PlotRequest) (*types.PlotResponse, error)
}
