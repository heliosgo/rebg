package server

import (
	"fmt"
	"log"
	"net"
	"rebg/api"
	"strings"
)

type UDPServer struct {
	Key        string
	Conn       *net.UDPConn
	Port       int
	Stop       chan struct{}
	Notify     chan string
	Client     *net.UDPAddr
	User       *net.UDPAddr
	ClientHost string
}

func NewUDPServer(
	clientAddr string,
	item api.MessageRegisterItem,
	notify chan string,
	stop chan struct{},
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

		if s.User == nil && addr.IP.String() != s.ClientHost {
			log.Printf("new conn from user, key: %s\n", s.Key)
			s.User = addr
			s.Notify <- s.Key
			continue
		}
		if addr.IP.String() == s.ClientHost {
			s.Client = addr
		}
		if s.User != nil && s.Client != nil {
			s.startTunnel(s.User, s.Client, buf[:n])
		}
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
		user = nil
		client = nil
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
