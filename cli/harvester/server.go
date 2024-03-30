// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package harvester

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/krobertson/chia-garden/pkg/types"

	"github.com/dustin/go-humanize"
)

const (
	taintTransfers = 50 * time.Millisecond
	taintFreeSpace = 50 * time.Millisecond
)

var (
	systemHostname, _ = os.Hostname()
)

type harvester struct {
	plots       map[string]*plotPath
	sortedPlots []*plotPath
	sortMutex   sync.Mutex
	hostPort    string
	transfers   atomic.Int64
	httpServer  *http.Server
}

// newHarvester will create a the harvester server process and validate all of
// the provided plot paths. It will return an error if any of the paths do not
// exist, or are not a directory.
func newHarvester(paths []string) (*harvester, error) {
	hostport := fmt.Sprintf("%s:%d", httpServerIP, httpServerPort)
	h := &harvester{
		plots:       make(map[string]*plotPath),
		sortedPlots: make([]*plotPath, 0),
		hostPort:    hostport,
	}
	log.Printf("Using http://%s for transfers...", hostport)

	// validate the plots exist and add them in
	for _, p := range paths {
		p, err := filepath.Abs(p)
		if err != nil {
			log.Printf("Path %s failed expansion, skipping: %v", p, err)
			continue
		}

		fi, err := os.Stat(p)
		if err != nil {
			log.Printf("Path %s failed validation, skipping: %v", p, err)
			continue
		}

		if !fi.IsDir() {
			log.Printf("Path %s is not a directory, skipping", p)
			continue
		}

		pp := &plotPath{path: p}
		pp.updateFreeSpace()
		h.plots[p] = pp
		h.sortedPlots = append(h.sortedPlots, pp)

		log.Printf("Registred plot path: %s [%s free / %s total]",
			p, humanize.IBytes(pp.freeSpace), humanize.IBytes(pp.totalSpace))
	}

	// ensure we have at least one
	if len(h.sortedPlots) == 0 {
		return nil, fmt.Errorf("at least one valid plot path must be specified")
	}

	// sort the paths
	h.sortPaths()

	// set up the http server
	h.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", httpServerPort),
		Handler: http.DefaultServeMux,
	}
	http.HandleFunc("/", h.httpHandler)
	go h.httpServer.ListenAndServe()

	return h, nil
}

// PlotReady processes a request from a plotter to transfer a plot to the
// harvester. It will generate a response, but then momentarily sleep as a
// slight taint to allow the most ideal system to originally respond the
// fastest.
func (h *harvester) PlotReady(req *types.PlotRequest) (*types.PlotResponse, error) {
	// pick a plot. This should return the one with the most free space that
	// isn't busy.
	plot := h.pickPlot()
	if plot == nil {
		return nil, fmt.Errorf("no paths available")
	}

	// check if we have enough free space
	if plot.freeSpace <= req.Size {
		return nil, nil
	}

	// generate response
	resp := &types.PlotResponse{
		Hostname: systemHostname,
		Store:    plot.path,
		Url:      fmt.Sprintf("http://%s%s", h.hostPort, filepath.Join(plot.path, req.Name)),
	}

	// generate and handle the taint
	d := h.generateTaint(plot)
	log.Printf("Responding to plot request after %s taint", d.String())
	time.Sleep(d)
	return resp, nil
}

// PlotLocate is used to check if any harvesters have the specified plot. This
// is primarily used when a plotter is starting up and has some existing plots
// present. Returns a nil PlotLocateResponse if the plot does not exist.
func (h *harvester) PlotLocate(req *types.PlotLocateRequest) (*types.PlotLocateResponse, error) {
	for k := range h.plots {
		fullpath := filepath.Join(k, req.Name)
		fi, err := os.Stat(fullpath)

		// if it returns a not exists error, continue on looping
		if os.IsNotExist(err) {
			continue
		}

		// check other errors
		if err != nil {
			log.Printf("Error checking for file %s: %v", fullpath, err)
		}

		// check the size to see if it is a match
		if fi.Size() == int64(req.Size) {
			return &types.PlotLocateResponse{
				Hostname: systemHostname,
			}, nil
		}

		log.Printf("IMPORTANT: Processed PlotLocate request for %q and sizes did not match. Check validity of the plot file.", fullpath)
	}

	return nil, nil
}

// httpHandler faciliates the transfer of plot files from the plotters to the
// harvesters. It encapculates a single request and is ran within its own
// goroutine. It will respond with a 201 on success and a relevant error code on
// failure. A failure should trigger the plotter to re-request storage.
func (h *harvester) httpHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	// get the plot path and ensure it exists
	base := filepath.Dir(req.URL.Path)
	plotPath, exists := h.plots[base]
	if !exists {
		log.Printf("Request to store in %s, but does not exist", base)
		w.WriteHeader(404)
		return
	}

	// check if we're maxed on concurrent transfers
	if h.transfers.Load() >= maxTransfers {
		log.Printf("Request to store in %s, but at max transfers", base)
		w.WriteHeader(503)
		return
	}

	// make sure the disk isn't already being written to. this helps to avoid
	// file fragmentation
	if plotPath.busy.Load() {
		log.Printf("Request to store %s, but already trasnferring", req.URL.Path)
		w.WriteHeader(503)
		return
	}

	// make sure we have the content length
	if req.ContentLength == 0 {
		w.WriteHeader(411)
		return
	}

	// lock the file path
	plotPath.mutex.Lock()
	defer plotPath.mutex.Unlock()
	plotPath.busy.Store(true)
	defer plotPath.busy.Store(false)
	h.transfers.Add(1)
	defer h.transfers.Add(-1)

	// check if we have enough free space
	if plotPath.freeSpace <= uint64(req.ContentLength) {
		log.Printf("Request to store %s, but not enough space (%s / %s)",
			req.URL.Path, humanize.Bytes(uint64(req.ContentLength)), humanize.Bytes(plotPath.freeSpace))
		w.WriteHeader(413)
	}

	// validate the file doesn't already exist, as a safeguard
	fi, _ := os.Stat(req.URL.Path)
	if fi != nil {
		log.Printf("File at %s already exists", req.URL.Path)
		w.WriteHeader(409)
		return
	}

	// open the file and transfer
	tmpfile := req.URL.Path + ".tmp"
	os.Remove(tmpfile)
	f, err := os.Create(tmpfile)
	if err != nil {
		log.Printf("Failed to open file at %s: %v", tmpfile, err)
		w.WriteHeader(500)
		plotPath.pause()
		return
	}
	defer f.Close()

	// perform the copy
	log.Printf("Receiving plot at %s", req.URL.Path)
	start := time.Now()
	bytes, err := io.Copy(f, req.Body)
	if err != nil {
		log.Printf("Failure while writing plot %s: %v", tmpfile, err)
		f.Close()
		os.Remove(tmpfile)
		w.WriteHeader(500)
		plotPath.pause()
		return
	}

	// rename it so it can be used by the chia harvester
	err = os.Rename(tmpfile, req.URL.Path)
	if err != nil {
		log.Printf("Failed to rename final plot %s: %v", req.URL.Path, err)
		f.Close()
		os.Remove(tmpfile)
		w.WriteHeader(500)
		plotPath.pause()
		return
	}

	// log successful and some metrics
	seconds := time.Since(start).Seconds()
	log.Printf("Successfully stored %s (%s, %f secs, %s/sec)",
		req.URL.Path, humanize.IBytes(uint64(bytes)), seconds, humanize.Bytes(uint64(float64(bytes)/seconds)))
	w.WriteHeader(201)

	// update free space
	plotPath.updateFreeSpace()
	h.sortPaths()
}

// generateTaint will calculate how long to delay the response based on current
// system pressure. This can be used to organically load balance in a cluster,
// allowing more preferencial hosts to respond faster.
func (h *harvester) generateTaint(plot *plotPath) time.Duration {
	d := time.Duration(0)

	// apply per current transfer going on. this helps prefer harvesters with
	// less busy networks
	d += time.Duration(h.transfers.Load()) * taintTransfers

	// apply for ratio of free disk space. this prefers harvesters with emptier
	// disks
	percent := 100 - (100 * plot.freeSpace / plot.totalSpace)
	d += time.Duration(percent) * taintFreeSpace / 100

	return d
}
