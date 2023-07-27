package errorx

import "errors"

var (
	ErrClientRegisterFailed = errors.New("client register failed")
	ErrUnknownMessageType   = errors.New("unknown message type")
	ErrUnknownProtocolType  = errors.New("unknown protocol type")
	ErrNotFoundConn         = errors.New("not found conn")
)
