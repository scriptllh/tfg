/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

// +build darwin netbsd freebsd openbsd dragonfly linux

package tfg

import (
	"golang.org/x/sys/unix"
	"sync/atomic"
)

type pollEvent struct {
	id        int
	connCount int64
	poll      *poll
	s         *server
}

func (e *pollEvent) incConnCount() {
	atomic.AddInt64(&e.connCount, 1)
}

func (e *pollEvent) decConnCount() {
	atomic.AddInt64(&e.connCount, -1)
}

func (e *pollEvent) run() {
	defer func() {
		e.s.signalShutdown()
		e.s.wg.Done()
	}()
	e.poll.wait(func(fd int, mode int32) error {
		c, _ := e.s.connManager.get(fd)
		if c == nil {
			return e.accept(fd)
		}
		if mode == 'r' || mode == 'r'+'w' {
			e.read(c)
		}
		if mode == 'w' || mode == 'r'+'w' {
			e.write()
		}
		return nil
	})
}

func (e *pollEvent) opened(c *conn) {
	c.setConnOpened()
	if e.s.handleConn.PreOpen != nil {
		e.s.handleConn.PreOpen(c)
	}
}

func (e *pollEvent) write() {
	e.s.setAvailableWrite()
}

func (e *pollEvent) accept(fd int) error {
	for {
		if len(e.s.pollEvents) > 1 {
			switch e.s.acceptBalance {
			case RoundRobin:
				if e.id != int(atomic.LoadInt64(&e.s.connManager.connCount))%e.s.numPollEvent {
					return nil
				}
			case LeastConn:
				count := atomic.LoadInt64(&e.connCount)
				for _, event := range e.s.pollEvents {
					if count > atomic.LoadInt64(&event.connCount) {
						return nil
					}
				}
			}
		}
		nfd, sa, err := unix.Accept(fd)
		if err != nil {
			if err == unix.EAGAIN {
				return nil
			}
			return err
		}
		if err := unix.SetNonblock(nfd, true); err != nil {
			return err
		}
		conn := e.s.connManager.connCache.Get().(*conn)
		conn.fd = nfd
		conn.sa = sa
		conn.laddr = e.s.ln.lnaddr
		conn.s = e.s
		conn.indexPollEvent = e.id
		conn.raddr = conn.saToAddr(sa)
		e.s.connManager.add(conn.fd, conn)
		e.poll.addFd(conn.fd)
		e.incConnCount()
		e.s.connManager.incConnCount()
		e.opened(conn)
	}
	return nil
}

func (e *pollEvent) read(c *conn) {
	for {
		cw := e.s.connManager.inCache.Get().(*connWorker)
		n, err := unix.Read(c.fd, cw.in)
		if n == 0 || err != nil {
			if err == unix.EAGAIN {
				return
			}
			c.setConnNeedClosed()
			return
		}
		cw.conn = c
		c.s.poolHandle.handleConn(cw)
	}
}
