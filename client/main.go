package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"rebg/api"
	"rebg/errorx"
)

type Client struct {
	Conf       Config
	ServerConn net.Conn
}

func NewClient(file string) (*Client, error) {
	conf, err := NewConfig(file)
	if err != nil {
		return nil, err
	}

	return &Client{Conf: conf}, nil
}

func (c *Client) Start() error {
	addr := fmt.Sprintf("%s:%d", c.Conf.Core.RemoteHost, c.Conf.Core.RemotePort)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("connect server success, addr: %s\n", addr)
	c.ServerConn = conn

	if err := c.register(); err != nil {
		return err
	}

	log.Printf("client is started\n")

	return c.waitConnect()
}

func (c *Client) register() error {
	msg := api.Message{
		Type: api.MessageTypeRegister,
	}
	data := api.MessageRegister{
		M: make(map[string]api.MessageRegisterItem),
	}
	for k, v := range c.Conf.Items {
		data.M[k] = api.MessageRegisterItem{
			Key:        k,
			Type:       api.ProtocolType(v.Type),
			RemotePort: v.RemotePort,
		}
	}
	msg.Data = data
	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.ServerConn.Write(msgByte)
	if err != nil {
		return err
	}

	respByte := make([]byte, 1024)
	n, err := c.ServerConn.Read(respByte)
	if err != nil {
		return err
	}
	var respMsg api.Message
	err = json.Unmarshal(respByte[:n], &respMsg)
	if err != nil {
		return err
	}
	if respMsg.Type != api.MessageTypeRegister ||
		respMsg.Type == api.MessageTypeRegister &&
			respMsg.Data.(string) != api.MessageDataOK {
		return errorx.ErrClientRegisterFailed
	}

	log.Printf("register successed\n")

	return nil
}

func (c *Client) waitConnect() error {
	defer c.ServerConn.Close()
	for {
		msgByte := make([]byte, 1024)
		n, err := c.ServerConn.Read(msgByte)
		if err != nil {
			return err
		}
		var msg api.Message
		err = json.Unmarshal(msgByte[:n], &msg)
		if err != nil {
			return err
		}
		log.Printf("new conn comming, msg: %v\n", msg)
		dataByte, err := json.Marshal(msg.Data)
		if err != nil {
			return err
		}
		var data api.MessageConnect
		if err := json.Unmarshal(dataByte, &data); err != nil {
			return err
		}
		go func() {
			if err := c.handleConnect(data); err != nil {
				fmt.Println(err)
			}
		}()
	}
}

func (c *Client) handleConnect(data api.MessageConnect) error {
	item := c.Conf.Items[data.LocalKey]
	localAddr := fmt.Sprintf("%s:%d", item.LocalHost, item.LocalPort)
	remoteAddr := fmt.Sprintf("%s:%d", c.Conf.Core.RemoteHost, item.RemotePort)

	switch item.Type {
	case api.ProtocolTypeTCP:
		return c.handleTCPConnect(localAddr, remoteAddr)
	case api.ProtocolTypeHTTP:
		return c.handleTCPConnect(localAddr, remoteAddr)
	case api.ProtocolTypeUDP:
		return c.handleUDPConnect(localAddr, remoteAddr)
	}

	return nil
}

func (c *Client) handleTCPConnect(localAddr, remoteAddr string) error {
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		return err
	}
	log.Printf("connect local success, addr: %s\n", localAddr)
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		return err
	}
	log.Printf("connect remote success, addr: %s\n", remoteAddr)

	go c.joinConnect(localConn, remoteConn)

	return nil
}

func (c *Client) handleUDPConnect(localAddr, remoteAddr string) error {
	lraddr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return err
	}
	rraddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return err
	}
	lconn, err := net.DialUDP("udp", nil, lraddr)
	if err != nil {
		return err
	}
	defer lconn.Close()
	rconn, err := net.DialUDP("udp", nil, rraddr)
	if err != nil {
		return err
	}
	defer rconn.Close()
	// first msg
	rconn.Write([]byte("ok"))
	lread, rread := make(chan []byte), make(chan []byte)
	go c.readFromUDP(lconn, lread)
	go c.readFromUDP(rconn, rread)
	for {
		select {
		case msg := <-lread:
			rconn.Write(msg)
		case msg := <-rread:
			lconn.Write(msg)
		}
	}
}

func (c *Client) readFromUDP(conn *net.UDPConn, ch chan []byte) {
	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf(
				"read from udp %s failed, err: %v\n",
				conn.RemoteAddr(), err,
			)
			return
		}
		ch <- buf[:n]
	}
}

func (c *Client) joinConnect(local, remote net.Conn) {
	defer func() {
		local.Close()
		remote.Close()
	}()

	lread, rread := make(chan []byte), make(chan []byte)
	go c.read(local, lread)
	go c.read(remote, rread)
	for {
		select {
		case msg := <-lread:
			log.Printf("read from local: %s\n", msg)
			remote.Write(msg)
		case msg := <-rread:
			log.Printf("read from remote: %s\n", msg)
			local.Write(msg)
		}
	}
}

func (c *Client) read(conn net.Conn, ch chan []byte) {
	defer conn.Close()
	for {
		msg := make([]byte, 1024)
		n, err := conn.Read(msg)
		if err != nil {
			log.Printf("read from %s failed, err: %v\n", conn.RemoteAddr(), err)
			return
		}

		ch <- msg[:n]
	}
}
