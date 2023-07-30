package server

import (
	"fmt"
	"log"
	"net"
	"rebg/api"
	"strings"
)

type HTTPServer struct {
	Key        string
	Listener   net.Listener
	Port       int
	Stop       chan struct{}
	Notify     chan string
	Client     net.Conn
	User       net.Conn
	ClientHost string
}

func NewHTTPServer(
	clientAddr string,
	item api.MessageRegisterItem,
	notify chan string,
	stop chan struct{},
) (*HTTPServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", item.RemotePort))
	if err != nil {
		return nil, err
	}

	return &HTTPServer{
		Key:        item.Key,
		Listener:   listener,
		Notify:     notify,
		Port:       item.RemotePort,
		Stop:       stop,
		ClientHost: strings.Split(clientAddr, ":")[0],
	}, nil
}

func (s *HTTPServer) Run() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			log.Printf("accept new conn failed, err: %v\n", err)
			continue
		}
		log.Printf("new conn from %s\n", conn.RemoteAddr())

		host := strings.Split(conn.RemoteAddr().String(), ":")[0]
		if s.User == nil && host != s.ClientHost {
			log.Printf("new conn from user, key: %s\n", s.Key)
			s.User = conn
			s.Notify <- s.Key
			continue
		}
		if host == s.ClientHost {
			s.Client = conn
		}
		if s.User != nil && s.Client != nil {
			s.startTunnel(s.User, s.Client)
		}
	}
}

func (s *HTTPServer) startTunnel(user, client net.Conn) {
	log.Printf(
		"start tunnel, user: %s, client: %s\n",
		user.RemoteAddr(), client.RemoteAddr(),
	)
	s.joinConnect(user, client)
}

func (s *HTTPServer) joinConnect(user, client net.Conn) {
	defer func() {
		user.Close()
		client.Close()
	}()

	aread, bread := make(chan []byte), make(chan []byte)
	go s.read(user, aread)
	go s.read(client, bread)
	for {
		select {
		case msg := <-aread:
			log.Printf("read from user: %s\n", msg)
			client.Write(msg)
		case msg := <-bread:
			log.Printf("read from client: %s\n", msg)
			user.Write(msg)
		}
	}
}
func (s *HTTPServer) read(conn net.Conn, ch chan []byte) {
	defer conn.Close()
	for {
		msg := make([]byte, 1024)
		n, err := conn.Read(msg)
		if err != nil {
			log.Printf("read from client failed, err: %v\n", err)
			return
		}

		ch <- msg[:n]
	}
}
