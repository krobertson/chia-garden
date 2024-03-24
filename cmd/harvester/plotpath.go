package main

import (
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

type plotPath struct {
	path       string
	busy       atomic.Bool
	freeSpace  uint64
	totalSpace uint64
	mutex      sync.Mutex
}

// updateFreeSpace will get the filesystem stats and update the free and total
// space on the plotPath. This primarily should be done with the plotPath mutex
// locked.
func (p *plotPath) updateFreeSpace() {
	var stat unix.Statfs_t
	unix.Statfs(p.path, &stat)

	p.freeSpace = stat.Bavail * uint64(stat.Bsize)
	p.totalSpace = stat.Blocks * uint64(stat.Bsize)
}
