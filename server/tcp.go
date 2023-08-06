package server

import (
	"fmt"
	"log"
	"net"
	"rebg/api"
	"strings"
	"sync"
)

type TCPServer struct {
	Key        string
	Listener   net.Listener
	Port       int
	Stop       chan struct{}
	Notify     chan string
	Conns      map[string]*Connx
	ClientHost string
	Close      chan Close
	Queue      []net.Conn

	Mutex sync.RWMutex
}

type Connx struct {
	Client net.Conn
	User   net.Conn
}

func NewTCPServer(
	clientAddr string,
	item api.MessageRegisterItem,
	notify chan string,
	stop chan struct{},
	closeCh chan Close,
) (*TCPServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", item.RemotePort))
	if err != nil {
		return nil, err
	}

	return &TCPServer{
		Key:        item.Key,
		Listener:   listener,
		Notify:     notify,
		Port:       item.RemotePort,
		Stop:       stop,
		ClientHost: strings.Split(clientAddr, ":")[0],
		Conns:      make(map[string]*Connx),
		Close:      closeCh,
		Queue:      make([]net.Conn, 0),
	}, nil
}

func (s *TCPServer) Run() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			log.Printf("accept new conn failed, err: %v\n", err)
			continue
		}
		log.Printf("new conn from %s\n", conn.RemoteAddr())

		host := strings.Split(conn.RemoteAddr().String(), ":")[0]
		if host != s.ClientHost {
			log.Printf("new conn from user, key: %s\n", s.Key)
			s.Queue = append(s.Queue, conn)
			s.Notify <- s.Key
			continue
		}
		if len(s.Queue) == 0 {
			conn.Close()
			continue
		}
		s.Mutex.Lock()
		connx := &Connx{
			User:   s.Queue[0],
			Client: conn,
		}
		s.Conns[conn.RemoteAddr().String()] = connx
		s.Mutex.Unlock()
		s.Queue = s.Queue[1:]
		go s.startTunnel(connx.User, connx.Client)
	}
}

func (s *TCPServer) startTunnel(user, client net.Conn) {
	log.Printf(
		"start tunnel, user: %s, client: %s\n",
		user.RemoteAddr(), client.RemoteAddr(),
	)
	s.joinConnect(user, client)
}

func (s *TCPServer) joinConnect(user, client net.Conn) {
	defer func() {
		s.Close <- Close{Key: s.Key, Addr: client.RemoteAddr().String()}
		user.Close()
		client.Close()
		s.Mutex.Lock()
		delete(s.Conns, client.RemoteAddr().String())
		s.Mutex.Unlock()
	}()

	aread, bread := make(chan []byte), make(chan []byte)
	go s.read(user, aread)
	go s.read(client, bread)
	for {
		select {
		case msg, ok := <-aread:
			if !ok {
				return
			}
			log.Printf("read from user: %s\n", msg)
			client.Write(msg)
		case msg, ok := <-bread:
			if !ok {
				return
			}
			log.Printf("read from client: %s\n", msg)
			user.Write(msg)
		case <-s.Stop:
			return
		}
	}
}
func (s *TCPServer) read(conn net.Conn, ch chan []byte) {
	for {
		msg := make([]byte, 1024)
		n, err := conn.Read(msg)
		if err != nil {
			log.Printf("read from client failed, err: %v\n", err)
			close(ch)
			return
		}

		ch <- msg[:n]
	}
}
