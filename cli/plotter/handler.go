// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package plotter

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/krobertson/chia-garden/pkg/types"

	"github.com/dustin/go-humanize"
)

var (
	failedPlots     = []string{}
	failedPlotMutex = sync.Mutex{}
)

func plotworker(client *rpc.NatsPlotterClient, ch chan string) {
	for plot := range ch {
		ok := handlePlot(client, plot)
		if !ok {
			// failed to send it, so requeue
			ch <- plot
		}
	}
}

// handlePlot will publish a message to find a location to send the plot. In
// some failure cases, like not finding a host to send it to, it will sleep for
// a minute and retry up to 10 times. The goal is that there may be transient
// issues or machines at capacity, so to slow down the sending. In the case of a
// failure on the other server end, it will retry immediately. If it is able to
// get the plot sent successfully, it will return true. If it is not, and it
// times out in retries, it will return false so the caller can requeue the
// plot.
func handlePlot(client *rpc.NatsPlotterClient, plot string) bool {
	// gather info
	fi, err := os.Stat(plot)
	if err != nil {
		log.Print("Failed to stat plot file", plot, err)
		return true
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
			return false
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
		log.Printf("Sending plot %s to %s:%s", plot, resp.Hostname, resp.Store)
		httpresp, err := http.DefaultTransport.RoundTrip(httpreq)
		if err != nil {
			log.Print("HTTP transfer failed", err)
			f.Close()
			time.Sleep(time.Minute)
			continue
		}

		switch httpresp.StatusCode {
		case 201: // success
			f.Close()
			seconds := time.Since(start).Seconds()
			log.Printf("Finished transfering plot %s (%s, %f secs, %s/sec)",
				plot, humanize.IBytes(uint64(req.Size)), seconds, humanize.Bytes(uint64(float64(req.Size)/seconds)))
			os.Remove(plot)
			return true

		case 500: // transfer failure due to server error, wait a minute and retry
			log.Print("Received 500 status code from server. Sleep and retry.", httpresp.Status)
			f.Close()
			time.Sleep(time.Minute)
			continue

		default: // other failures should immediately retry
			log.Printf("Received %d status code from server, retry ready request immediately.", httpresp.StatusCode)
			f.Close()
			continue
		}
	}

	// Too many retries, log and continue
	log.Printf("Timed out transferring plot file %s, will retry later or on next restart", plot)
	failedPlotMutex.Lock()
	failedPlots = append(failedPlots, plot)
	failedPlotMutex.Unlock()
	return false
}
