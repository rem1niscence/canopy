package types

import (
	"container/list"
	"time"
)

type SimpleLimiter struct {
	requests        map[string]int
	totalRequests   int
	maxPerRequester int
	maxRequests     int
	reset           *time.Ticker
}

func NewLimiter(maxPerRequester, maxRequests, resetWindowS int) *SimpleLimiter {
	return &SimpleLimiter{
		requests:        map[string]int{},
		maxPerRequester: maxPerRequester,
		maxRequests:     maxRequests,
		reset:           time.NewTicker(time.Duration(resetWindowS) * time.Second),
	}
}

func (l *SimpleLimiter) NewRequest(requester string) (requesterBlock, totalBlock bool) {
	if l.totalRequests >= l.maxRequests {
		return false, true
	}
	if count := l.requests[requester]; count >= l.maxPerRequester {
		return true, false
	}
	l.requests[requester]++
	l.totalRequests++
	return
}

func (l *SimpleLimiter) Reset() {
	l.requests = map[string]int{}
	l.totalRequests = 0
}

func (l *SimpleLimiter) C() <-chan time.Time {
	return l.reset.C
}

const (
	MaxMessageCacheSize = 10000
)

type MessageCache struct {
	queue *list.List
	m     map[string]struct{}
}

func NewMessageCache() MessageCache {
	return MessageCache{
		queue: list.New(),
		m:     map[string]struct{}{},
	}
}

func (c MessageCache) Add(msg *MessageWrapper) bool {
	k := BytesToString(msg.Hash)
	if _, found := c.m[k]; found {
		return false
	}
	if c.queue.Len() >= MaxMessageCacheSize {
		e := c.queue.Front()
		message := e.Value.(*MessageWrapper)
		delete(c.m, BytesToString(message.Hash))
		c.queue.Remove(e)
	}
	c.m[k] = struct{}{}
	c.queue.PushFront(msg)
	return true
}
