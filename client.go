package chanrpc

import (
	"errors"
	"fmt"
	logger "github.com/hezhis/go_log"
	"runtime"
)

type Client struct {
	s                *Server
	chanSyncRet      chan *RetInfo
	ChanAsyncRet     chan *RetInfo
	pendingAsyncCall int
}

func NewClient(l int) *Client {
	c := new(Client)
	c.chanSyncRet = make(chan *RetInfo, 1)
	c.ChanAsyncRet = make(chan *RetInfo, l)
	return c
}

func (c *Client) Attach(s *Server) {
	c.s = s
}

func (c *Client) f(id interface{}, n int) (f interface{}, err error) {
	if c.s == nil {
		err = errors.New("server not attached")
		return
	}

	f = c.s.router[id]
	if f == nil {
		err = fmt.Errorf("router id %v: router not registered", id)
		return
	}

	var ok bool
	switch n {
	case 0:
		_, ok = f.(func([]interface{}))
	case 1:
		_, ok = f.(func([]interface{}) interface{})
	case 2:
		_, ok = f.(func([]interface{}) []interface{})
	default:
		panic("bug")
	}

	if !ok {
		err = fmt.Errorf("function id %v: return type mismatch", id)
	}
	return
}

func (c *Client) call(ci *CallInfo, block bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if block {
		c.s.ChanCall <- ci
	} else {
		select {
		case c.s.ChanCall <- ci:
		default:
			err = errors.New("chan rpc channel full")
		}
	}
	return
}

func (c *Client) asyncCall(id interface{}, args []interface{}, cb interface{}, n int) {
	f, err := c.f(id, n)
	if err != nil {
		c.ChanAsyncRet <- &RetInfo{err: err, cb: cb}
		return
	}

	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.ChanAsyncRet,
		cb:      cb,
	}, false)
	if err != nil {
		c.ChanAsyncRet <- &RetInfo{err: err, cb: cb}
		return
	}
}

func (c *Client) Call(id interface{}, args ...interface{}) {
	f, err := c.f(id, 0)
	if err != nil {
		logger.Error("chan rpc callback function not found! id:%v", id)
		return
	}

	err = c.call(&CallInfo{
		f:       f,
		args:    args,
		chanRet: c.ChanAsyncRet,
		cb:      nil,
	}, false)
	if err != nil {
		logger.Error("chan rpc callback function error! id:%v, err:%v", id, err)
		return
	}
}

func (c *Client) AsyncCall(id interface{}, _args ...interface{}) {
	if len(_args) < 1 {
		logger.Error("callback function not found")
		return
	}

	args := _args[:len(_args)-1]
	cb := _args[len(_args)-1]

	var n int
	switch cb.(type) {
	case func(error):
		n = 0
	case func(interface{}, error):
		n = 1
	case func([]interface{}, error):
		n = 2
	default:
		logger.Error("definition of callback function is invalid! id:%%v", id)
		return
	}

	// too many calls
	if c.pendingAsyncCall >= cap(c.ChanAsyncRet) {
		execCb(&RetInfo{err: errors.New("too many calls"), cb: cb})
		return
	}

	c.asyncCall(id, args, cb, n)

	c.pendingAsyncCall++
}

func assert(i interface{}) []interface{} {
	if i == nil {
		return nil
	} else {
		return i.([]interface{})
	}
}
func execCb(ri *RetInfo) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 1024)
			l := runtime.Stack(buf, false)
			logger.Error("%v: %s", r, buf[:l])
		}
	}()

	// execute
	switch ri.cb.(type) {
	case func(error):
		ri.cb.(func(error))(ri.err)
	case func(interface{}, error):
		ri.cb.(func(interface{}, error))(ri.ret, ri.err)
	case func([]interface{}, error):
		ri.cb.(func([]interface{}, error))(assert(ri.ret), ri.err)
	default:
		panic("bug")
	}
	return
}
func (c *Client) Cb(ri *RetInfo) {
	c.pendingAsyncCall--
	execCb(ri)
}

func (c *Client) Close() {
	for c.pendingAsyncCall > 0 {
		c.Cb(<-c.ChanAsyncRet)
	}
}

func (c *Client) Idle() bool {
	return c.pendingAsyncCall == 0
}
