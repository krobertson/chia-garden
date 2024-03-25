// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package plotter

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/krobertson/chia-garden/pkg/types"
)

var (
	failedPlots     = []string{}
	failedPlotMutex = sync.Mutex{}
)

func plotworker(client *rpc.NatsPlotterClient, ch <-chan string) {
	for plot := range ch {
		handlePlot(client, plot)
	}
}

func handlePlot(client *rpc.NatsPlotterClient, plot string) {
	// gather info
	fi, err := os.Stat(plot)
	if err != nil {
		log.Print("Failed to stat plot file", plot, err)
		return
	}
	req := &types.PlotRequest{
		Name: filepath.Base(plot),
		Size: uint64(fi.Size()),
	}

	for i := 0; i < 10; i++ {
		resp, err := client.PlotReady(req)
		if err != nil {
			log.Print("Received error on plot ready request", err)
			time.Sleep(time.Minute)
			continue
		}

		// if we did not get a response, sleep and try again
		if resp == nil {
			log.Print("Received no response")
			time.Sleep(time.Minute)
			continue
		}

		// open the file
		f, err := os.Open(plot)
		if err != nil {
			log.Print("Failed to open plot file, bailing", err)
			return
		}

		// if we got a response, dispatch the transfer
		httpreq, err := http.NewRequest("POST", resp.Url, f)
		if err != nil {
			log.Print("Failed to open http request", err)
			f.Close()
			time.Sleep(time.Minute)
			continue
		}
		httpreq.ContentLength = int64(req.Size)

		start := time.Now()
		log.Printf("Sending plot %s to %s/%s", plot, resp.Hostname, resp.Store)
		httpresp, err := http.DefaultTransport.RoundTrip(httpreq)
		if err != nil {
			log.Print("HTTP transfer failed", err)
			f.Close()
			time.Sleep(time.Minute)
			continue
		}

		// FIXME better handle various error codes
		if httpresp.StatusCode != 201 {
			log.Print("Received bad response", httpresp.StatusCode, httpresp.Status)
			f.Close()
			time.Sleep(time.Minute)
			continue
		}

		// all done now
		f.Close()
		seconds := time.Since(start).Seconds()
		log.Printf("Finished transfering plot %s (%s, %f secs, %s/sec)",
			plot, humanize.IBytes(uint64(req.Size)), seconds, humanize.Bytes(uint64(float64(req.Size)/seconds)))
		os.Remove(plot)
		return
	}

	// Too many retries, log and continue
	log.Printf("Timed out transferring plot file %s, will retry later or on next restart", plot)
	failedPlotMutex.Lock()
	failedPlots = append(failedPlots, plot)
	failedPlotMutex.Unlock()
}
