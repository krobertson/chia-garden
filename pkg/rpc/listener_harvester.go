// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package rpc

import (
	"encoding/json"
	"log"

	"github.com/krobertson/chia-garden/pkg/types"
	"github.com/nats-io/nats.go"
)

type NatsHarvesterListener struct {
	client  *nats.Conn
	handler Harvester
}

func NewNatsHarvesterListener(client *nats.Conn, handler Harvester) (*NatsHarvesterListener, error) {
	w := &NatsHarvesterListener{
		client:  client,
		handler: handler,
	}
	return w, w.RegisterHandlers()
}

func (w *NatsHarvesterListener) RegisterHandlers() error {
	_, err := w.client.Subscribe(subjPlotReady, w.handlerPlot)
	if err != nil {
		return err
	}

	return w.client.Flush()
}

func (d *NatsHarvesterListener) handlerPlot(msg *nats.Msg) {
	var req *types.PlotRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Println("Failed to unmarshal rig")
		return
	}

	resp, err := d.handler.PlotReady(req)
	if resp != nil || err != nil {
		d.respond(msg, resp, err)
	}
}

func (d *NatsHarvesterListener) respond(msg *nats.Msg, v interface{}, err error) {
	// if there is no reply subject, just bail, we have no option
	if msg.Reply == "" {
		if err != nil {
			log.Printf("Error returned from call: %v", err)
		}
		return
	}

	resp := &natsResponse{}
	if err != nil {
		s := err.Error()
		resp.Error = &s
	} else {
		data, err := json.Marshal(v)
		if err != nil {
			s := err.Error()
			resp.Error = &s
		} else {
			resp.Result = data
		}
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to generate reply message: %v", err)
		return
	}

	if err = d.client.Publish(msg.Reply, data); err != nil {
		log.Printf("Failed to publish reply message: %v", err)
		return
	}
}
