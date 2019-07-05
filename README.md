<h1 align="center">Welcome to tfg 👋</h1>



### 🏠 [Homepage](https://github.com/scriptllh/tfg)

## Install

```sh
go get -u github.com/scriptllh/tfg
```


## Usage

```sh
package main

import (
	"fmt"
	"github.com/scriptllh/tfg"
	"time"
)

type HandleConn struct {
	tfg.BaseHandleConn
}

func (hc *HandleConn) PreOpen(c tfg.Conn) {
	fmt.Println("pre conn open", c)
}

type req struct {
	s string
}

func (hc *HandleConn) Read(in []byte, lastRemain []byte) (packet interface{}, remain []byte, isFinRead bool, isHandle bool) {
	s := string(in)
	req := &req{
		s: s,
	}
	fmt.Println("read: ", req)
	return req, nil, false, true
}

func (hc *HandleConn) Handle(conn tfg.Conn, packet interface{}) {
	req := packet.(*req)
	fmt.Println("handle req", req)
	time.Sleep(time.Millisecond * time.Duration(10))
	n, err := conn.Write([]byte("tfg la la la"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("handle write:", n)
}

func main() {
	var handleConn HandleConn
	s, err := tfg.NewServer(":6000", &handleConn, 0, tfg.RoundRobin)
	if err != nil {
		return
	}
	if err := s.Serve(); err != nil {
		return
	}
}

```

## Run

```sh
  git clone https://github.com/scriptllh/tfg-example.git
  make dev
```



## 🤝 Contributing

Contributions, issues and feature requests are welcome!<br />Feel free to check [issues page](https://github.com/scriptllh/tfg/issues).

## Show your support

Give a ⭐️ if this project helped you!



