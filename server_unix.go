/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

// +build darwin netbsd freebsd openbsd dragonfly linux

package tfg

import (
	"golang.org/x/sys/unix"
	"runtime"
)

func (s *server) serving() error {
	if err := unix.SetNonblock(s.ln.fd, true); err != nil {
		return err
	}
	if s.numPollEvent <= 0 {
		s.numPollEvent = runtime.NumCPU()
	}
	defer func() {
		s.waitForShutdown()
		s.Stop()
	}()
	for id := 0; id < s.numPollEvent; id++ {
		poll, err := mkPoll()
		if err != nil {
			return err
		}
		event := &pollEvent{
			id:   id,
			poll: poll,
			s:    s,
		}
		event.poll.addLn(s.ln.fd)
		s.pollEvents = append(s.pollEvents, event)
	}
	s.wg.Add(len(s.pollEvents))
	for _, pollEvent := range s.pollEvents {
		go pollEvent.run()
	}
	return nil
}

func (s *server) waitForShutdown() {
	s.cond.L.Lock()
	s.cond.Wait()
	s.cond.L.Unlock()
}

func (s *server) signalShutdown() {
	s.cond.L.Lock()
	s.cond.Signal()
	s.cond.L.Unlock()
}
