/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 * 对 https://github.com/panjf2000/ants 进行了适当的修改以满足数据异步处理的连接池
 */

package tfg

import (
	"log"
	"time"
)

type WorkerHandle struct {
	pool *PoolHandle

	connCh chan *connWorker

	recycleTime time.Time
}

type connWorker struct {
	conn *conn
	in   []byte
	n    int
}

func (w *WorkerHandle) run() {
	w.pool.incRunning()
	go func() {
		defer func() {
			if p := recover(); p != nil {
				w.pool.decRunning()
				w.pool.workerCache.Put(w)
				if w.pool.PanicHandler != nil {
					w.pool.PanicHandler(p)
				} else {
					log.Printf("worker exits from a panic: %v", p)
				}
			}
		}()

		var remain []byte
		var packet interface{}
		var isFinRead, isHandle bool
		var err error
		for connWorker := range w.connCh {
			if nil == connWorker || connWorker.conn == nil {
				w.pool.decRunning()
				w.pool.workerCache.Put(w)
				return
			}
			packet, remain, isFinRead, isHandle, err = w.pool.read(connWorker.in[:connWorker.n], remain)
			if isHandle {
				req := w.pool.s.connManager.handleReqCache.Get().(*handleReq)
				req.conn = connWorker.conn
				req.packet = packet
				req.err = err
				w.pool.s.pool.Serve(req)
			}
			if isFinRead {
				w.pool.s.connManager.inCache.Put(connWorker)
				if ok := w.pool.revertWorker(w); !ok {
					break
				}
				w.pool.deleteConnWorker(connWorker.conn.fd)
			}
		}
	}()
}
