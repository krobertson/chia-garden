// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package rpc

import (
	"github.com/krobertson/chia-garden/pkg/types"
	"github.com/nats-io/nats.go"
)

type NatsPlotterClient struct {
	client *nats.Conn
}

func NewNatsPlotterClient(conn *nats.Conn) *NatsPlotterClient {
	return &NatsPlotterClient{
		client: conn,
	}
}

func (d *NatsPlotterClient) PlotReady(plot *types.PlotRequest) (*types.PlotResponse, error) {
	var resp *types.PlotResponse
	if err := request(d.client, subjPlotReady, plot, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
