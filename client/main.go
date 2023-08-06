package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"rebg/api"
	"rebg/errorx"
	"sync"
)

type Client struct {
	Conf       Config
	ServerConn net.Conn
	LocalConn  map[string]net.Conn
	Mutex      sync.RWMutex
}

func NewClient(file string) (*Client, error) {
	conf, err := NewConfig(file)
	if err != nil {
		return nil, err
	}

	return &Client{
		Conf:      conf,
		LocalConn: make(map[string]net.Conn),
	}, nil
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
		preHandleMsg := c.preHandleMessage(msgByte[:n])
		for _, v := range preHandleMsg {
			var msg api.Message
			err = json.Unmarshal(v, &msg)
			if err != nil {
				return err
			}
			log.Printf("new msg comming, msg: %v\n", msg)
			go func() {
				if err := c.handleMessage(msg); err != nil {
					log.Printf("handle message failed, err: %v\n", err)
				}
			}()
		}
	}
}

func (c *Client) preHandleMessage(msg []byte) [][]byte {
	res := make([][]byte, 0)
	stack := make([]int, 0)
	for i, v := range msg {
		switch v {
		case '{':
			stack = append(stack, i)
		case '}':
			l := len(stack)
			if l == 0 {
				continue
			}
			if msg[stack[l-1]] == '{' {
				if l == 1 {
					res = append(res, msg[stack[l-1]:i+1])
				}
				stack = stack[:l-1]
			}
		}
	}

	return res
}

func (c *Client) handleMessage(msg api.Message) error {
	dataByte, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	switch msg.Type {
	case api.MessageTypeConnect:
		var data api.MessageConnect
		if err := json.Unmarshal(dataByte, &data); err != nil {
			return err
		}
		c.handleConnect(data)
	case api.MessageTypeClose:
		var data api.MessageClose
		if err := json.Unmarshal(dataByte, &data); err != nil {
			return err
		}
		c.handleClose(data)
	}

	return nil
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

func (c *Client) handleClose(data api.MessageClose) error {
	c.Mutex.Lock()
	c.LocalConn[data.Addr].Close()
	delete(c.LocalConn, data.Addr)
	c.Mutex.Unlock()

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

	c.Mutex.Lock()
	c.LocalConn[remoteConn.LocalAddr().String()] = localConn
	c.Mutex.Unlock()

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
	c.Mutex.Lock()
	c.LocalConn[rconn.LocalAddr().String()] = lconn
	c.Mutex.Unlock()
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
