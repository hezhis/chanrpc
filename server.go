package chanrpc

import (
	"fmt"
	logger "github.com/hezhis/go_log"
	"runtime"
)

type (
	Server struct {
		router   map[interface{}]interface{}
		ChanCall chan *CallInfo
	}
)

func NewServer(cap int) *Server {
	s := &Server{}
	s.router = make(map[interface{}]interface{})
	s.ChanCall = make(chan *CallInfo, cap)
	return s
}

func (s *Server) Open(l int) *Client {
	c := NewClient(l)
	c.Attach(s)
	return c
}

func (s *Server) Register(id interface{}, f interface{}) {
	switch f.(type) {
	case func([]interface{}):
	case func([]interface{}) interface{}:
	case func([]interface{}) []interface{}:
	default:
		logger.Fatal("chan rpc id %v: definition of router is invalid", id)
	}

	if _, ok := s.router[id]; ok {
		logger.Fatal("chan rpc router id %v: already registered", id)
	}

	s.router[id] = f
}

func (s *Server) ret(ci *CallInfo, ri *RetInfo) (err error) {
	if ci.chanRet == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	ri.cb = ci.cb
	ci.chanRet <- ri
	return
}

func (s *Server) exec(ci *CallInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 1024)
			l := runtime.Stack(buf, false)
			err = fmt.Errorf("%v: %s", r, buf[:l])

			s.ret(ci, &RetInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	// execute
	switch ci.f.(type) {
	case func([]interface{}):
		ci.f.(func([]interface{}))(ci.args)
		return s.ret(ci, &RetInfo{})
	case func([]interface{}) interface{}:
		ret := ci.f.(func([]interface{}) interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	case func([]interface{}) []interface{}:
		ret := ci.f.(func([]interface{}) []interface{})(ci.args)
		return s.ret(ci, &RetInfo{ret: ret})
	}

	panic("bug")
}

func (s *Server) Exec(ci *CallInfo) {
	err := s.exec(ci)
	if err != nil {
		logger.Error("%v", err)
	}
}
