// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package rpc

import (
	"time"

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
	if err := request(d.client, subjPlotReady, plot, &resp, time.Second*5); err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *NatsPlotterClient) PlotLocate(plot *types.PlotLocateRequest) (*types.PlotLocateResponse, error) {
	var resp *types.PlotLocateResponse
	if err := request(d.client, subjPlotLocate, plot, &resp, time.Second*1); err != nil {
		return nil, err
	}
	return resp, nil
}
