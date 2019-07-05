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
)

type listener struct {
	ln     net.Listener
	fd     int
	lnaddr net.Addr
	s      *server
}

func (l *listener) ok() bool { return l != nil && l.ln != nil }

func (l *listener) Close() error {
	if !l.ok() {
		return unix.EINVAL
	}
	if err := l.ln.Close(); err != nil {
		return err
	}
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.ln.Addr()
}
