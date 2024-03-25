// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package harvester

import (
	"cmp"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/krobertson/chia-garden/pkg/types"

	"github.com/dustin/go-humanize"
)

const (
	taintTransfers = 20 * time.Millisecond
	taintFreeSpace = 20 * time.Millisecond
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
}

// newHarvester will create a the harvester server process and validate all of
// the provided plot paths. It will return an error if any of the paths do not
// exist, or are not a directory.
func newHarvester(paths []string) (*harvester, error) {
	hostport := fmt.Sprintf("%s:%d", httpServerIP, httpServerPort)
	h := &harvester{
		plots:       make(map[string]*plotPath),
		sortedPlots: make([]*plotPath, len(paths)),
		hostPort:    hostport,
	}
	log.Printf("Using http://%s for transfers...", hostport)

	// ensure we have at least one
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one plot path must be specified")
	}

	// validate the plots exist and add them in
	for i, p := range paths {
		p, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("path %s failed expansion: %v", p, err)
		}

		fi, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("path %s failed validation: %v", p, err)
		}

		if !fi.IsDir() {
			return nil, fmt.Errorf("path %s is not a directory", p)
		}

		pp := &plotPath{path: p}
		pp.updateFreeSpace()
		h.plots[p] = pp
		h.sortedPlots[i] = pp
	}

	// sort the paths
	h.sortPaths()

	// FIXME ideally handle graceful shutdown of existing transfers
	http.HandleFunc("/", h.httpHandler)
	go http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), nil)

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
		Url: fmt.Sprintf("http://%s%s", h.hostPort, filepath.Join(plot.path, req.Name)),
	}

	// generate and handle the taint
	d := h.generateTaint(plot)
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
	f, err := os.Create(req.URL.Path)
	if err != nil {
		log.Printf("Failed to open file at %s: %v", req.URL.Path, err)
		w.WriteHeader(500)
		return
	}
	defer f.Close()

	// perform the copy
	start := time.Now()
	bytes, err := io.Copy(f, req.Body)
	if err != nil {
		log.Printf("Failure while writing plot %s: %v", req.URL.Path, err)
		f.Close()
		os.Remove(req.URL.Path)
		w.WriteHeader(500)
		return
	}

	// update free space
	plotPath.updateFreeSpace()
	h.sortPaths()

	// log successful and some metrics
	seconds := time.Since(start).Seconds()
	log.Printf("Successfully stored %s (%s, %f secs, %s/sec)",
		req.URL.Path, humanize.IBytes(uint64(bytes)), seconds, humanize.Bytes(uint64(float64(bytes)/seconds)))
	w.WriteHeader(201)
}

// sortPaths will update the order of the plotPaths inside the harvester's
// sortedPaths slice. This should be done after every file transfer when the
// free space is updated.
func (h *harvester) sortPaths() {
	h.sortMutex.Lock()
	defer h.sortMutex.Unlock()

	slices.SortStableFunc(h.sortedPlots, func(a, b *plotPath) int {
		return cmp.Compare(a.freeSpace, b.freeSpace)
	})
}

// pickPlot will return which plot path would be most ideal for the current
// request. It will order the one with the most free space that doesn't already
// have an active transfer.
func (h *harvester) pickPlot() *plotPath {
	h.sortMutex.Lock()
	defer h.sortMutex.Unlock()

	for _, v := range h.sortedPlots {
		if v.busy.Load() {
			continue
		}
		return v
	}
	return nil
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
	percent := 100 * plot.freeSpace / plot.totalSpace
	d += time.Duration(percent) * taintFreeSpace / 1000

	return d
}
