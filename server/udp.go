package server

import (
	"fmt"
	"log"
	"net"
	"rebg/api"
	"strings"
	"sync"
)

type UDPServer struct {
	Key        string
	Conn       *net.UDPConn
	Port       int
	Stop       chan struct{}
	Notify     chan string
	Conns      map[string]*UDPConnx
	ClientHost string
	Close      chan Close
	Queue      []*net.UDPAddr

	Mutex sync.RWMutex
}

type UDPConnx struct {
	User   *net.UDPAddr
	Client *net.UDPAddr
}

func NewUDPServer(
	clientAddr string,
	item api.MessageRegisterItem,
	notify chan string,
	stop chan struct{},
	closeCh chan Close,
) (*UDPServer, error) {
	addr := fmt.Sprintf(":%d", item.RemotePort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	return &UDPServer{
		Key:        item.Key,
		Conn:       conn,
		Notify:     notify,
		Port:       item.RemotePort,
		Stop:       stop,
		ClientHost: strings.Split(clientAddr, ":")[0],
		Conns:      make(map[string]*UDPConnx),
		Close:      closeCh,
		Queue:      make([]*net.UDPAddr, 0),
	}, nil
}

func (s *UDPServer) Run() {
	buf := make([]byte, 1024)
	for {
		n, addr, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("accept new conn failed, err: %v\n", err)
			continue
		}
		log.Printf("new conn from %s\n", addr)
		if addr.IP.String() != s.ClientHost {
			log.Printf("new conn from user, key: %s\n", s.Key)
			s.Queue = append(s.Queue, addr)
			s.Notify <- s.Key
			continue
		}
		if len(s.Queue) == 0 {
			continue
		}

		s.Mutex.Lock()
		connx := &UDPConnx{
			User:   s.Queue[0],
			Client: addr,
		}
		s.Conns[addr.AddrPort().String()] = connx
		s.Mutex.Unlock()
		s.Queue = s.Queue[1:]
		go s.startTunnel(connx.User, connx.Client, buf[:n])
	}
}

func (s *UDPServer) startTunnel(user, client *net.UDPAddr, firstMsg []byte) {
	log.Printf(
		"start tunnel, user: %s, client: %s\n",
		user.String(), client.String(),
	)
	_, err := s.Conn.WriteTo(firstMsg, client)
	if err != nil {
		log.Printf("write first msg: [%s] failed, err: %v\n", firstMsg, err)
	}
	s.joinConnect(user, client)
}

func (s *UDPServer) joinConnect(user, client *net.UDPAddr) {
	defer func() {
		s.Close <- Close{Key: s.Key, Addr: client.AddrPort().String()}
		s.Mutex.Lock()
		delete(s.Conns, client.AddrPort().String())
		s.Mutex.Unlock()
	}()
	buf := make([]byte, 1024)
	for {
		n, addr, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		switch addr.String() {
		case user.String():
			s.Conn.WriteTo(buf[:n], client)
		case client.String():
			s.Conn.WriteTo(buf[:n], user)
		}
	}
}
