package rpc

import (
	"sync"

	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
)

const defaultIndexerBlobCacheEntries = 64

type indexerBlobCacheEntry struct {
	height     uint64
	blobs      *fsm.IndexerBlobs
	protoBytes []byte
}

type indexerBlobCache struct {
	mu         sync.RWMutex
	maxEntries int
	entries    map[uint64]*indexerBlobCacheEntry
	order      []uint64
}

func newIndexerBlobCache(maxEntries int) *indexerBlobCache {
	if maxEntries <= 0 {
		maxEntries = defaultIndexerBlobCacheEntries
	}
	return &indexerBlobCache{
		maxEntries: maxEntries,
		entries:    make(map[uint64]*indexerBlobCacheEntry),
		order:      make([]uint64, 0, maxEntries),
	}
}

func (c *indexerBlobCache) get(height uint64) (*indexerBlobCacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[height]
	c.mu.RUnlock()
	return entry, ok
}

func (c *indexerBlobCache) getCurrent(height uint64) (*fsm.IndexerBlob, bool) {
	entry, ok := c.get(height)
	if !ok || entry == nil || entry.blobs == nil {
		return nil, false
	}
	return entry.blobs.Current, entry.blobs.Current != nil
}

func (c *indexerBlobCache) put(height uint64, entry *indexerBlobCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[height]; ok {
		c.entries[height] = entry
		c.touch(height)
		return
	}

	c.entries[height] = entry
	c.order = append(c.order, height)
	if len(c.order) <= c.maxEntries {
		return
	}

	evictHeight := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, evictHeight)
}

func (c *indexerBlobCache) touch(height uint64) {
	for i, h := range c.order {
		if h == height {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, height)
			return
		}
	}
}

// IndexerBlobsCached builds and caches indexer blobs (current + previous) for a height.
func (r *RCManager) IndexerBlobsCached(height uint64) (*fsm.IndexerBlobs, []byte, lib.ErrorI) {
	currentHeight := r.controller.FSM.Height()
	if height == 0 || height > currentHeight {
		height = currentHeight
	}

	if entry, ok := r.indexerBlobCache.get(height); ok && entry != nil && entry.blobs != nil && entry.protoBytes != nil {
		return entry.blobs, entry.protoBytes, nil
	}

	current, err := r.controller.FSM.IndexerBlob(height)
	if err != nil {
		return nil, nil, err
	}

	var previous *fsm.IndexerBlob
	if height > 1 {
		if cachedPrev, ok := r.indexerBlobCache.getCurrent(height - 1); ok {
			previous = cachedPrev
		} else {
			prev, prevErr := r.controller.FSM.IndexerBlob(height - 1)
			if prevErr != nil {
				return nil, nil, prevErr
			}
			previous = prev
		}
	}

	blobs := &fsm.IndexerBlobs{
		Current:  current,
		Previous: previous,
	}
	protoBytes, err := lib.Marshal(blobs)
	if err != nil {
		return nil, nil, err
	}

	r.indexerBlobCache.put(height, &indexerBlobCacheEntry{
		height:     height,
		blobs:      blobs,
		protoBytes: protoBytes,
	})

	return blobs, protoBytes, nil
}
