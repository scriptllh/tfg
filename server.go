/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

package tfg

import (
	"errors"
	"github.com/panjf2000/ants"
	"math"
	"net"
	"sync"
	"sync/atomic"
)

type Server interface {
	Start() error
	Stop()
	Serve() error
	Listener() net.Listener
	SetPreServing(func(server Server))
}

type AcceptBalance int

const (
	Random AcceptBalance = iota
	RoundRobin
	LeastConn
)

var (
	defaultPoolHandleCleanIntervalTime = 1
	defaultInLen                       = 32
	defaultPoolHandleSize              = math.MaxInt32 / 2
	defaultPoolSize                    = math.MaxInt32 / 2
	defaultPoolCleanIntervalTime       = 5
	ErrInputConnWrite                  = errors.New("input err for conn write")
	ErrClosedPoll                      = errors.New("closed for poll")
)

func (s *server) Listener() net.Listener {
	return s.ln.ln
}

func (s *server) SetPreServing(f func(server Server)) {
	s.preServing = f
}

type server struct {
	pool          *ants.PoolWithFunc
	pollEvents    []*pollEvent
	wg            sync.WaitGroup
	cond          *sync.Cond
	acceptBalance AcceptBalance
	poolHandle    *PoolHandle
	numPollEvent  int
	ln            *listener
	addr          string
	preServing    func(server Server)
	handleConn    HandleConn
	isWrite       uint32
	connManager   *connManager
}

func (s *server) setAvailableWrite() {
	atomic.StoreUint32(&s.isWrite, AVAILABLEWRITE)
}

func (s *server) setUnAvailableWrite() {
	atomic.StoreUint32(&s.isWrite, UNAVAILABLEWRITE)
}

func NewServer(addr string, HandleConn HandleConn, numPollEvent int, acceptBalance AcceptBalance) (Server, error) {
	s := &server{
		addr:          addr,
		numPollEvent:  numPollEvent,
		cond:          sync.NewCond(&sync.Mutex{}),
		acceptBalance: acceptBalance,
		connManager: &connManager{
			conns: &sync.Map{},
			inCache: &sync.Pool{
				New: func() interface{} {
					return &connWorker{in: make([]byte, defaultInLen)}
				},
			},
			connCache: &sync.Pool{
				New: func() interface{} {
					return &conn{}
				},
			},
			handleReqCache: &sync.Pool{
				New: func() interface{} {
					return &handleReq{}
				},
			},
		},
		handleConn: HandleConn,
	}
	pool, err := ants.NewTimingPoolWithFunc(defaultPoolSize, defaultPoolCleanIntervalTime, func(i interface{}) {
		req := i.(*handleReq)
		defer func() {
			if req.conn.isNeedClose() {
				req.conn.Close()
			}
			s.connManager.handleReqCache.Put(req)
		}()
		s.handleConn.Handle(req.conn, req.packet)
	})
	if err != nil {
		return nil, err
	}
	s.pool = pool
	poolHandle, err := NewPoolHandle(defaultPoolHandleSize, s.handleConn.Read, s.handleConn.Handle, s)
	if err != nil {
		return nil, err
	}
	s.poolHandle = poolHandle
	return s, nil
}

func (s *server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	tcpListener := ln.(*net.TCPListener)
	lnFile, err := tcpListener.File()
	if err != nil {
		return err
	}
	s.ln = &listener{
		ln:     tcpListener,
		fd:     int(lnFile.Fd()),
		lnaddr: tcpListener.Addr(),
		s:      s,
	}
	return s.serving()
}

func (s *server) Stop() {
	for _, l := range s.pollEvents {
		l.poll.triggerClose()
	}
	s.wg.Wait()
	s.connManager.CloseAllConn()
	for _, l := range s.pollEvents {
		l.poll.close()
	}
	s.ln.Close()
	s.pool.Release()
	s.poolHandle.Release()
}
func (s *server) Serve() error {
	if s.preServing != nil {
		s.preServing(s)
	}

	if err := s.Start(); err != nil {
		return err
	}
	return nil
}
