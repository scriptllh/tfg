/**
 * @Author: llh
 * @Date:   2019-06-01 15:08:12
 * @Last Modified by:   llh
 */

package tfg

type HandleConn interface {
	PreOpen(c Conn)
	Read(in []byte, lastRemain []byte) (packet interface{}, remain []byte, isFinRead bool, isHandle bool, err error)
	Handle(conn Conn, packet interface{}, err error)
}

type BaseHandleConn struct {
}

func (hc *BaseHandleConn) PreOpen(c Conn) {

}

func (hc *BaseHandleConn) Read(in []byte, lastRemain []byte) (packet interface{}, remain []byte, isFinRead bool, isHandle bool, err error) {
	return nil, nil, true, false, nil
}

type handleReq struct {
	conn   Conn
	packet interface{}
	err    error
}

func (hc *BaseHandleConn) Handle(conn Conn, packet interface{}, err error) {
}
