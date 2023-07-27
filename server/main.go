package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"rebg/api"
	"rebg/errorx"
	"sync"
)

type Server struct {
	Mutex     sync.RWMutex
	Conf      Config
	Listener  net.Listener
	Sub       map[string]SubServer
	KeyToConn map[string]net.Conn
	Stop      chan struct{}
	Notify    chan string
	Read      chan api.Message
	Conn      chan net.Conn
}

func NewServer(conf Config) *Server {
	res := &Server{
		Conf:      conf,
		Sub:       make(map[string]SubServer),
		KeyToConn: make(map[string]net.Conn),
		Stop:      make(chan struct{}),
		Notify:    make(chan string),
		Read:      make(chan api.Message),
		Conn:      make(chan net.Conn),
	}

	return res
}

// wait client
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Conf.Port))
	if err != nil {
		return err
	}
	s.Listener = listener
	go s.accept()
	for {
		select {
		case conn := <-s.Conn:
			go s.process(conn)
		case key := <-s.Notify:
			s.notifyConn(key)
		}
	}
}

func (s *Server) accept() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			log.Printf("accept new conn failed, err: %v\n", err)
			continue
		}
		s.Conn <- conn
	}
}

func (s *Server) process(conn net.Conn) {
	go s.read(conn)
	for {
		select {
		case msg := <-s.Read:
			s.handleMessage(conn, msg)
		}
	}
}

func (s *Server) handleMessage(conn net.Conn, msg api.Message) error {
	switch msg.Type {
	case api.MessageTypeRegister:
		return s.handleRegisterMessage(conn, msg.Data.(api.MessageRegister))
	}

	return errorx.ErrUnknownMessageType
}

func (s *Server) handleRegisterMessage(conn net.Conn, data api.MessageRegister) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	for k, v := range data.M {
		if _, ok := s.Sub[k]; ok {
			log.Printf("%s is registered, detail: %+v\n", k, v)
			continue
		}
		subServer, err := s.newSubServer(v)
		if err != nil {
			log.Printf("cannot build new sub server, key: %s, detail: %+v\n", k, v)
			continue
		}
		s.Sub[k] = subServer
		s.KeyToConn[k] = conn
		go subServer.Run()
	}

	return nil
}

func (s *Server) newSubServer(item api.MessageRegisterItem) (SubServer, error) {
	switch item.Type {
	case api.ProtocolTypeTCP:
		return NewTCPServer(item.RemotePort, s.Notify, s.Stop)
	}

	return nil, errorx.ErrUnknownProtocolType
}

func (s *Server) read(conn net.Conn) {
	for {
		msgByte := make([]byte, 1024)
		n, err := conn.Read(msgByte)
		if err != nil {
			log.Printf("read from client failed, err: %v\n", err)
			continue
		}
		var msg api.Message
		if err := json.Unmarshal(msgByte[:n], &msg); err != nil {
			log.Printf("json unmarshal failed, err: %v\n", err)
			continue
		}

		s.Read <- msg
	}
}

func (s *Server) notifyConn(key string) error {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	conn, ok := s.KeyToConn[key]
	if !ok {
		return errorx.ErrNotFoundConn
	}
	msg := api.MessageConnect{
		LocalKey: key,
	}
	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Write(msgByte)

	return err
}
