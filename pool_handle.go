/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 * 对 https://github.com/panjf2000/ants 进行了适当的修改以满足数据异步处理的连接池
 */

package tfg

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	CLOSED = 1
)

var (
	ErrInvalidPoolSize = errors.New("invalid size for pool")

	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")

	ErrPoolClosed = errors.New("this pool has been closed")

	workerChanCap = func() int {
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}
		return 1
	}()
)

type PoolHandle struct {
	s *server

	connWorkers sync.Map

	capacity int32

	running int32

	expiryDuration time.Duration

	workers []*WorkerHandle

	release int32

	lock sync.Mutex

	cond *sync.Cond

	read func(in []byte, lastRemain []byte) (packet interface{}, remain []byte, isFinRead bool, isHandle bool, err error)

	handle func(conn Conn, packet interface{}, err error)

	once sync.Once

	workerCache sync.Pool

	PanicHandler func(interface{})
}

func (p *PoolHandle) getConnWorker(fd int) (*WorkerHandle, bool) {
	v, ok := p.connWorkers.Load(fd)
	if !ok {
		return nil, ok
	}
	return v.(*WorkerHandle), true
}

func (p *PoolHandle) storeConnWorker(fd int, workHandle *WorkerHandle) {
	p.connWorkers.Store(fd, workHandle)
}

func (p *PoolHandle) deleteConnWorker(fd int) {
	p.connWorkers.Delete(fd)
}

func (p *PoolHandle) periodicallyPurge() {
	heartbeat := time.NewTicker(p.expiryDuration)
	defer heartbeat.Stop()

	for range heartbeat.C {
		if CLOSED == atomic.LoadInt32(&p.release) {
			break
		}
		currentTime := time.Now()
		p.lock.Lock()
		idleWorkers := p.workers
		n := -1
		for i, w := range idleWorkers {
			if currentTime.Sub(w.recycleTime) <= p.expiryDuration {
				break
			}
			n = i
			w.connCh <- nil
			idleWorkers[i] = nil
		}
		if n > -1 {
			if n >= len(idleWorkers)-1 {
				p.workers = idleWorkers[:0]
			} else {
				p.workers = idleWorkers[n+1:]
			}
		}
		p.lock.Unlock()
	}
}

func NewPoolHandle(size int, read func(in []byte, lastRemain []byte) (packet interface{},
	remain []byte, isFinRead bool, isHandle bool, err error), handle func(conn Conn, packet interface{}, err error), s *server) (*PoolHandle, error) {
	return NewTimingPoolHandle(size, defaultPoolHandleCleanIntervalTime, read, handle, s)
}

func NewTimingPoolHandle(size, expiry int, read func(in []byte, lastRemain []byte) (packet interface{},
	remain []byte, isFinRead bool, isHandle bool, err error), handle func(conn Conn, packet interface{}, err error), s *server) (*PoolHandle, error) {
	if size <= 0 {
		return nil, ErrInvalidPoolSize
	}
	if expiry <= 0 {
		return nil, ErrInvalidPoolExpiry
	}
	p := &PoolHandle{
		capacity:       int32(size),
		expiryDuration: time.Duration(expiry) * time.Second,
		read:           read,
		handle:         handle,
		s:              s,
	}
	p.cond = sync.NewCond(&p.lock)
	go p.periodicallyPurge()
	return p, nil
}

func (p *PoolHandle) handleConn(connWorker *connWorker) error {
	if CLOSED == atomic.LoadInt32(&p.release) {
		return ErrPoolClosed
	}
	var worker *WorkerHandle
	var ok bool
	worker, ok = p.getConnWorker(connWorker.conn.fd)
	if !ok {
		worker = p.retrieveWorker(connWorker.conn.fd)
		p.storeConnWorker(connWorker.conn.fd, worker)
	}
	worker.connCh <- connWorker
	return nil
}

func (p *PoolHandle) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *PoolHandle) Free() int {
	return int(atomic.LoadInt32(&p.capacity) - atomic.LoadInt32(&p.running))
}

func (p *PoolHandle) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

func (p *PoolHandle) Release() error {
	p.once.Do(func() {
		atomic.StoreInt32(&p.release, 1)
		p.lock.Lock()
		idleWorkers := p.workers
		for i, w := range idleWorkers {
			w.connCh <- nil
			idleWorkers[i] = nil
		}
		p.workers = nil
		p.lock.Unlock()
	})
	return nil
}

func (p *PoolHandle) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

func (p *PoolHandle) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

func (p *PoolHandle) retrieveWorker(fd int) *WorkerHandle {
	var w *WorkerHandle
	p.lock.Lock()
	idleWorkers := p.workers
	n := len(idleWorkers) - 1
	if n >= 0 {
		w = idleWorkers[n]
		idleWorkers[n] = nil
		p.workers = idleWorkers[:n]
		p.lock.Unlock()
	} else if p.Running() < p.Cap() {
		p.lock.Unlock()
		if cacheWorker := p.workerCache.Get(); cacheWorker != nil {
			w = cacheWorker.(*WorkerHandle)
		} else {
			w = &WorkerHandle{
				pool:   p,
				connCh: make(chan *connWorker, workerChanCap),
			}
		}
		w.run()
	} else {
		for {
			p.cond.Wait()
			l := len(p.workers) - 1
			if l < 0 {
				continue
			}
			w = p.workers[l]
			p.workers[l] = nil
			p.workers = p.workers[:l]
			break
		}
		p.lock.Unlock()
	}
	return w
}

func (p *PoolHandle) revertWorker(worker *WorkerHandle) bool {
	if CLOSED == atomic.LoadInt32(&p.release) {
		return false
	}
	worker.recycleTime = time.Now()
	p.lock.Lock()
	p.workers = append(p.workers, worker)
	p.cond.Signal()
	p.lock.Unlock()
	return true
}
