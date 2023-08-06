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
	Close     chan Close
	Read      chan api.Message
	Conn      chan net.Conn
}

type Close struct {
	Key  string
	Addr string
}

func NewServer(file string) (*Server, error) {
	conf, err := NewConfig(file)
	if err != nil {
		return nil, err
	}

	res := &Server{
		Conf:      conf,
		Sub:       make(map[string]SubServer),
		KeyToConn: make(map[string]net.Conn),
		Stop:      make(chan struct{}),
		Notify:    make(chan string),
		Close:     make(chan Close),
		Read:      make(chan api.Message),
		Conn:      make(chan net.Conn),
	}

	return res, nil
}

// wait client
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Conf.Port))
	if err != nil {
		return err
	}
	s.Listener = listener
	go s.accept()
	log.Printf("server is started\n")
	for {
		select {
		case conn := <-s.Conn:
			go s.process(conn)
		case key := <-s.Notify:
			err := s.notifyConn(key)
			if err != nil {
				log.Printf("notify failed, key: %s, err: %v\n", key, err)
			}
		case c := <-s.Close:
			s.closeConn(c)
			if err != nil {
				log.Printf("close failed, c: %v, err: %v\n", c, err)
			}
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
		return s.handleRegisterMessage(conn, msg.Data)
	}

	return errorx.ErrUnknownMessageType
}

func (s *Server) handleRegisterMessage(conn net.Conn, src any) error {
	var data api.MessageRegister
	srcByte, err := json.Marshal(src)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(srcByte, &data); err != nil {
		return err
	}
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	for k, v := range data.M {
		if _, ok := s.Sub[k]; ok {
			log.Printf("%s is registered, detail: %+v\n", k, v)
			continue
		}
		subServer, err := s.newSubServer(conn.RemoteAddr().String(), v)
		if err != nil {
			log.Printf("cannot build new sub server, key: %s, detail: %+v\n", k, v)
			continue
		}
		s.Sub[k] = subServer
		s.KeyToConn[k] = conn
		go subServer.Run()
	}

	resp := api.Message{
		Type: api.MessageTypeRegister,
		Data: api.MessageDataOK,
	}
	respByte, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	conn.Write(respByte)

	return nil
}

func (s *Server) newSubServer(clientAddr string, item api.MessageRegisterItem) (SubServer, error) {
	switch item.Type {
	case api.ProtocolTypeTCP:
		return NewTCPServer(clientAddr, item, s.Notify, s.Stop, s.Close)
	case api.ProtocolTypeHTTP:
		return NewHTTPServer(clientAddr, item, s.Notify, s.Stop, s.Close)
	case api.ProtocolTypeUDP:
		return NewUDPServer(clientAddr, item, s.Notify, s.Stop, s.Close)
	}

	return nil, errorx.ErrUnknownProtocolType
}

func (s *Server) read(conn net.Conn) {
	defer conn.Close()
	for {
		msgByte := make([]byte, 1024)
		n, err := conn.Read(msgByte)
		if err != nil {
			log.Printf("read from client failed, err: %v\n", err)
			return
		}
		var msg api.Message
		if err := json.Unmarshal(msgByte[:n], &msg); err != nil {
			log.Printf("json unmarshal failed, err: %v\n", err)
			return
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
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}
	msg := api.Message{
		Type: api.MessageTypeConnect,
		Data: api.MessageConnect{
			LocalKey: key,
		},
	}
	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Write(msgByte)

	return err
}

func (s *Server) closeConn(c Close) error {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	conn, ok := s.KeyToConn[c.Key]
	if !ok {
		return errorx.ErrNotFoundConn
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}
	msg := api.Message{
		Type: api.MessageTypeClose,
		Data: api.MessageClose{
			Addr: c.Addr,
		},
	}
	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Write(msgByte)

	return err
}
