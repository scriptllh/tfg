/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

// +build darwin netbsd freebsd openbsd dragonfly linux
package tfg

import (
	"golang.org/x/sys/unix"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	AVAILABLEWRITE uint32 = iota
	UNAVAILABLEWRITE
	CONN_CLOSE uint32 = iota
	CONN_OPEN
	CONN_NEEE_CLOSED
)

type Conn interface {
	Write(b []byte) (int, error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	isNeedClose() bool
}

type conn struct {
	fd             int
	sa             unix.Sockaddr // remote socket address
	laddr          net.Addr
	raddr          net.Addr
	s              *server
	indexPollEvent int
	status         uint32
	once           sync.Once
}

func (c *conn) setConnOpened() {
	atomic.StoreUint32(&c.status, CONN_OPEN)
}

func (c *conn) setConnNeedClosed() {
	atomic.StoreUint32(&c.status, CONN_NEEE_CLOSED)
}

func (c *conn) setConnClosed() {
	atomic.StoreUint32(&c.status, CONN_CLOSE)
}

func (c *conn) isNeedClose() bool {
	if atomic.LoadUint32(&c.status) == CONN_CLOSE {
		return true
	}
	return false
}

func (c *conn) ok() bool { return c != nil && c.fd != 0 }

func (c *conn) Write(b []byte) (int, error) {
	if b == nil || len(b) == 0 {
		return 0, ErrInputConnWrite
	}
	var n int
	var err error
	for {
		if atomic.LoadUint32(&c.s.isWrite) == AVAILABLEWRITE {
			n, err = unix.Write(c.fd, b)
			if err != nil {
				if err == syscall.EAGAIN {
					c.s.setUnAvailableWrite()
					continue
				}
				return 0, c.Close()
			}
			break
		}
	}
	return n, nil
}

func (c *conn) Close() error {
	var err error
	c.once.Do(func() {
		c.s.connManager.delete(c.fd)
		pollEvent := c.s.pollEvents[c.indexPollEvent]
		pollEvent.poll.remove(c.fd)
		if err = unix.Close(c.fd); err != nil {
			return
		}
		c.s.connManager.decConnCount()
		pollEvent.decConnCount()
		c.setConnClosed()
	})
	return err
}

func (c *conn) LocalAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.laddr
}

func (c *conn) RemoteAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.raddr
}

func (c *conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *conn) saToAddr(sa unix.Sockaddr) net.Addr {
	var a net.Addr
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		a = &net.TCPAddr{
			IP:   append([]byte{}, sa.Addr[:]...),
			Port: sa.Port,
		}
	case *unix.SockaddrInet6:
		var zone string
		if sa.ZoneId != 0 {
			if ifi, err := net.InterfaceByIndex(int(sa.ZoneId)); err == nil {
				zone = ifi.Name
			}
		}
		if zone == "" && sa.ZoneId != 0 {
		}
		a = &net.TCPAddr{
			IP:   append([]byte{}, sa.Addr[:]...),
			Port: sa.Port,
			Zone: zone,
		}
	}
	return a
}
