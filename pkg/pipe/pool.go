package pipe

import (
	"container/list"
	"github.com/iberryful/sproxy/pkg/log"
	"sync"
	"time"
)

type Pool struct {
	mu            sync.Mutex
	l             list.List
	maxAge        time.Duration
	maxSize       int
	lowWarterMark int
	gcInerval     time.Duration
	New           func() (*Pipe, error)
}

func NewPool(maxSize int, maxAge time.Duration, newFunc func() (*Pipe, error)) *Pool {
	p := &Pool{
		mu:            sync.Mutex{},
		l:             list.List{},
		maxAge:        maxAge,
		maxSize:       maxSize,
		lowWarterMark: max(8, maxSize>>1),
		gcInerval:     maxAge >> 2,
		New:           newFunc,
	}
	go p.gcLoop()
	return p
}

func (pool *Pool) Len() int {
	return pool.l.Len()
}

func (pool *Pool) Get() (*Pipe, error) {
	pool.mu.Lock()
	pool.cleanup()
	if pool.l.Len() > 0 {
		elem := pool.l.Front()
		p := elem.Value.(*Pipe)
		pool.mu.Unlock()
		return p, nil
	}
	pool.mu.Unlock()
	return pool.New()
}

func (pool *Pool) Put(p *Pipe) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for pool.l.Len() >= pool.maxSize {
		elem := pool.l.Front()
		p := elem.Value.(*Pipe)
		p.Close()
		pool.l.Remove(elem)
	}

	p.lastActive = time.Now()
	pool.l.PushBack(p)
}

// Lock required
func (pool *Pool) cleanup() int {
	r := 0
	for pool.l.Len() > 0 {
		elem := pool.l.Front()
		p := elem.Value.(*Pipe)
		pool.l.Remove(elem)
		if time.Now().Sub(p.lastActive) < pool.maxAge {
			return r
		}
		r += 1
		p.Close()
	}
	return r
}

func (pool *Pool) gc() {
	pool.mu.Lock()
	r := pool.cleanup()
	n := max(0, pool.lowWarterMark-pool.l.Len())
	pool.mu.Unlock()
	for i := 0; i < n; i++ {
		go func() {
			p, err := pool.New()
			if err == nil {
				pool.Put(p)
			}
		}()
	}
	log.Infof("[GC loop]removed %d pipes, added %d pipes", r, n)
}

func (pool *Pool) gcLoop() {
	log.Info("start gc loop")
	pool.gc()
	for range time.Tick(pool.gcInerval) {
		pool.gc()
	}
}
