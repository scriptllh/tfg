/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

package tfg

import (
	"sync"
	"sync/atomic"
)

type ConnManger interface {
	Len() int64
	CloseAllConn()
}

type connManager struct {
	conns     *sync.Map
	connCount int64
	connCache *sync.Pool
	inCache   *sync.Pool
}

func (m *connManager) getIn() {
}

func (m *connManager) incConnCount() {
	atomic.AddInt64(&m.connCount, 1)
}

func (m *connManager) decConnCount() {
	atomic.AddInt64(&m.connCount, -1)
}

func (m *connManager) delete(fd int) {
	m.conns.Delete(fd)
}

func (m *connManager) get(fd int) (*conn, bool) {
	v, ok := m.conns.Load(fd)
	if !ok {
		return nil, ok
	}
	return v.(*conn), true
}

func (m *connManager) add(key int, v Conn) {
	m.conns.Store(key, v)
}

func (m *connManager) Len() int64 {
	return m.connCount
}

func (m *connManager) CloseAllConn() {
	m.conns.Range(func(key, value interface{}) bool {
		conn := value.(Conn)
		conn.Close()
		return true
	})
}
