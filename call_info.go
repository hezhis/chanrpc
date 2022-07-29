package chanrpc

type (
	RetInfo struct {
		ret interface{}
		err error
		cb  interface{}
	}

	CallInfo struct {
		f       interface{}
		args    []interface{}
		chanRet chan *RetInfo
		cb      interface{}
	}
)
