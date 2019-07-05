/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

// +build linux

package tfg

import (
	"golang.org/x/sys/unix"
	"sync/atomic"
)

const (
	POLL_OPENED uint32 = iota
	POLL_CLOSED
)

type poll struct {
	fd     int
	status uint32
}

func mkPoll() (*poll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &poll{
		fd: fd,
	}, nil
}

func (p *poll) triggerClose() {
	atomic.AddUint32(&p.status, POLL_CLOSED)
}

func (p *poll) close() error {
	return unix.Close(p.fd)
}

func (p *poll) wait(f func(fd int, mode int32) error) error {
	events := make([]unix.EpollEvent, 64)
	for {
		n, err := unix.EpollWait(p.fd, events, -1)
		if err != nil && err != unix.EINTR {
			return err
		}
		if atomic.LoadUint32(&p.status) == POLL_CLOSED {
			return ErrClosedPoll
		}
		for i := 0; i < n; i++ {
			var mode int32
			if events[i].Events&(unix.EPOLLIN|unix.EPOLLRDHUP|unix.EPOLLHUP|unix.EPOLLERR) != 0 {
				mode += 'r'
			}
			if events[i].Events&(unix.EPOLLOUT|unix.EPOLLHUP|unix.EPOLLERR) != 0 {
				mode += 'w'
			}
			if mode != 0 {
				if fd := int(events[i].Fd); fd != 0 {
					if err := f(fd, mode); err != nil {
						return err
					}
				}
			}
		}
	}
}

func (p *poll) addLn(fd int) {
	if err := unix.EpollCtl(p.fd, unix.EPOLL_CTL_ADD, fd,
		&unix.EpollEvent{Fd: int32(fd),
			Events: unix.EPOLLIN | unix.EPOLLET,
		},
	); err != nil {
		panic(err)
	}
}

func (p *poll) addFd(fd int) {
	if err := unix.EpollCtl(p.fd, unix.EPOLL_CTL_ADD, fd,
		&unix.EpollEvent{Fd: int32(fd),
			Events: unix.EPOLLIN | unix.EPOLLOUT | unix.EPOLLPRI | unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLET,
		},
	); err != nil {
		panic(err)
	}
}

func (p *poll) remove(fd int) {
	if err := unix.EpollCtl(p.fd, unix.EPOLL_CTL_DEL, fd, nil); err != nil {
		panic(err)
	}
}
