package server

import (
	"fmt"
	"io"
	"log"
	"net"
)

type TCPServer struct {
	Key        string
	Listener   net.Listener
	Port       int
	Stop       chan struct{}
	Notify     chan string
	Client     net.Conn
	User       net.Conn
	ClientAddr string
}

type Conns struct {
	Client net.Conn
	User   net.Conn
}

func NewTCPServer(port int, notify chan string, stop chan struct{}) (*TCPServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	return &TCPServer{
		Listener: listener,
		Port:     port,
		Stop:     stop,
	}, nil
}

func (s *TCPServer) Run() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			log.Printf("accept new conn failed, err: %v\n", err)
			continue
		}

		addr := conn.RemoteAddr().String()
		if s.User == nil && addr != s.ClientAddr {
			s.User = conn
			s.Notify <- s.Key
			continue
		}
		if addr == s.ClientAddr {
			s.Client = conn
		}
		if s.User != nil && s.Client != nil {
			s.startTunnel(s.User, s.Client)
		}
	}
}

func (s *TCPServer) startTunnel(a, b net.Conn) {
	go s.joinConnect(a, b)
	go s.joinConnect(b, a)
	select {}
}

func (s *TCPServer) joinConnect(a, b net.Conn) {
	defer func() {
		a.Close()
		b.Close()
	}()
	_, err := io.Copy(a, b)
	if err != nil {
		log.Printf("copy from %s to %s failed\n", a.RemoteAddr(), b.RemoteAddr())
		return
	}
}
