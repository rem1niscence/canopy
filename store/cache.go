package store

import (
	"bytes"
	"github.com/ginchuco/ginchu/types"
	"sort"
)

type Op int64

const (
	Delete Op = iota
	Set
)

type CacheObject struct {
	key       []byte
	value     []byte
	operation Op
}

type storeCache struct {
	unsortedCache map[string]CacheObject // used for maintaining operations
	sortedCache   [][]byte               // used for iterating (slice of keys to unsorted cache)
	parent        types.StoreI           // the parent this store is caching
}

var _ types.StoreI = &storeCache{}

func NewStoreCache(parent types.StoreI) *storeCache {
	return &storeCache{
		unsortedCache: make(map[string]CacheObject),
		sortedCache:   make([][]byte, 0),
		parent:        parent,
	}
}

func (i *storeCache) Get(key []byte) ([]byte, error) {
	if cacheObj, ok := i.unsortedCache[string(key)]; ok {
		if cacheObj.operation == Delete {
			return nil, nil
		} else {
			return cacheObj.value, nil
		}
	}
	return i.parent.Get(key)
}

func (i *storeCache) Set(key, value []byte) error {
	return i.newWriteOp(key, value, Set)
}

func (i *storeCache) Delete(key []byte) error {
	return i.newWriteOp(key, nil, Delete)
}

func (i *storeCache) newWriteOp(key, value []byte, op Op) error {
	_, found := i.unsortedCache[string(key)]
	i.unsortedCache[string(key)] = CacheObject{
		key:       key,
		value:     value,
		operation: op,
	}
	if !found {
		// if found, no op as the key isn't changing
		// any write (including delete) must override
		// the parent value
		i.addToSortedCache(key)
	}
	return nil
}

func (i *storeCache) index(key []byte) int {
	return sort.Search(len(i.sortedCache), func(a int) bool { return bytes.Equal(i.sortedCache[a], key) })
}

func (i *storeCache) addToSortedCache(key []byte) {
	i.sortedCache = append(i.sortedCache, key)
	sort.Slice(i.sortedCache, func(x, y int) bool {
		return bytes.Compare(i.sortedCache[x], i.sortedCache[y]) < 0
	})
}

func (i *storeCache) Iterator(start, end []byte) (types.IteratorI, error) {
	parent, err := i.parent.Iterator(start, end)
	if err != nil {
		return nil, err
	}
	return NewCacheMergeIterator(parent, i.sortedCache, i.unsortedCache, false), nil
}

func (i *storeCache) ReverseIterator(start, end []byte) (types.IteratorI, error) {
	parent, err := i.parent.ReverseIterator(start, end)
	if err != nil {
		return nil, err
	}
	return NewCacheMergeIterator(parent, i.sortedCache, i.unsortedCache, true), nil
}

func (i *storeCache) Write() {
	for _, co := range i.unsortedCache { // TODO add batching for leveldb to make this op atomic
		switch co.operation {
		case Set:
			_ = i.parent.Set(co.key, co.value)
		case Delete:
			_ = i.parent.Delete(co.key)
		}
	}
	// Clear the cache
	i.unsortedCache = make(map[string]CacheObject)
	i.sortedCache = make([][]byte, 0)
}

func (i *storeCache) Close() error { return nil }

type cacheMergeIterator struct {
	parent        types.IteratorI
	sortedCache   [][]byte
	unsortedCache map[string]CacheObject
	cacheLen      int
	cacheIndex    int
	reverse       bool
}

func NewCacheMergeIterator(parent types.IteratorI, sortedCache [][]byte, unsortedCache map[string]CacheObject, reverse bool) types.IteratorI {
	l := len(sortedCache)
	cacheIndex := 0
	if reverse {
		cacheIndex = l - 1
	}
	sc := make([][]byte, len(sortedCache))
	uc := make(map[string]CacheObject, len(unsortedCache))
	copy(sc, sortedCache)
	types.MapCopy(uc, unsortedCache)
	return &cacheMergeIterator{
		parent:        parent,
		sortedCache:   sc,
		unsortedCache: uc,
		cacheLen:      l,
		cacheIndex:    cacheIndex,
		reverse:       reverse,
	}
}

func (c *cacheMergeIterator) Next() {
	switch c.IteratorState() {
	case Neither:
		return
	case Cache:
		c.cacheNext()
	case Parent:
		c.parent.Next()
	}
}

func (c *cacheMergeIterator) Key() (key []byte) {
	switch c.IteratorState() {
	case Cache:
		return c.sortedCache[c.cacheIndex]
	case Parent:
		return c.parent.Key()
	default:
		return nil
	}
}

func (c *cacheMergeIterator) Value() (value []byte) {
	switch c.IteratorState() {
	case Cache:
		co := c.unsortedCache[string(c.sortedCache[c.cacheIndex])]
		return co.value
	case Parent:
		return c.parent.Value()
	default:
		return nil
	}
}

func (c *cacheMergeIterator) Valid() bool {
	if !c.parent.Valid() && !c.cacheValid() {
		return false
	}
	if c.handleDeletedValue() {
		return c.Valid()
	}
	return true
}

func (c *cacheMergeIterator) Close() {
	c.parent.Close()
}

func (c *cacheMergeIterator) Error() error {
	panic("error is not implemented on cacheMergeIterator")
}

func (c *cacheMergeIterator) Domain() (start []byte, end []byte) {
	panic("domain is not implemented on cacheMergeIterator")
}

func (c *cacheMergeIterator) IteratorState() State {
	pValid, cValid := c.parent.Valid(), c.cacheValid()
	if !pValid && !cValid {
		return Neither
	}
	if !pValid {
		return Cache
	}
	if !cValid {
		return Parent
	}
	// Both are valid.  Compare keys.
	keyP, keyC := c.parent.Key(), c.sortedCache[c.cacheIndex]
	cmp := c.compare(keyP, keyC)
	switch cmp {
	case -1: // parent < cache
		return Cache
	case 0: // parent == cache
		return Cache
	case 1: // parent > cache
		return Parent
	default:
		panic("invalid compare result")
	}
}

func (c *cacheMergeIterator) compare(a, b []byte) int {
	if c.reverse {
		return bytes.Compare(a, b)
	}
	return bytes.Compare(a, b) * -1
}

func (c *cacheMergeIterator) cacheValid() bool {
	if !c.reverse {
		return c.cacheIndex < c.cacheLen
	} else {
		return c.cacheIndex > -1
	}
}

func (c *cacheMergeIterator) cacheNext() {
	if !c.reverse {
		c.cacheIndex += 1
	} else {
		c.cacheIndex -= 1
	}
	c.handleDeletedValue()
}

func (c *cacheMergeIterator) handleDeletedValue() (wasDeletedValue bool) {
	if c.cacheValid() {
		if c.unsortedCache[string(c.sortedCache[c.cacheIndex])].operation == Delete {
			c.cacheNext()
			return true
		}
	}
	return false
}

var _ types.IteratorI = (*cacheMergeIterator)(nil)

type State int

const (
	Neither State = iota
	Cache
	Parent
)
