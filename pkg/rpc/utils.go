package rpc

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	subjPlotReady  = "b4s.plot.ready"
	subjPlotLocate = "b4s.plot.locate"
)

type natsResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

func request(client *nats.Conn, subj string, in interface{}, out interface{}) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}

	msg, err := client.Request(subj, data, time.Second*5)
	if err != nil {
		return err
	}

	var resp *natsResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return errors.New(*resp.Error)
	}

	if out != nil {
		return json.Unmarshal(resp.Result, &out)
	}
	return nil
}
