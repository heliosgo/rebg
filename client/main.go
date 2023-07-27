package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"rebg/api"
	"rebg/errorx"
	"time"
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

	err = c.ServerConn.SetReadDeadline(time.Now().Add(30 * time.Second))
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

	return nil
}

func (c *Client) waitConnect() error {
	for {
		msgByte := make([]byte, 1024)
		n, err := c.ServerConn.Read(msgByte)
		if err != nil {
			return err
		}
		var msg api.Message
		err = json.Unmarshal(msgByte[:n], &msg)
		if err != nil {
			log.Printf("read connect msg failed, err: %v\n", err)
			continue
		}
		go c.handleConnect(msg.Data.(api.MessageConnect))
	}
}

func (c *Client) handleConnect(data api.MessageConnect) error {
	item := c.Conf.Items[data.LocalKey]
	localAddr := fmt.Sprintf("%s:%d", item.LocalHost, item.LocalPort)
	remoteAddr := fmt.Sprintf("%s:%d", c.Conf.Core.RemoteHost, item.RemotePort)

	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		return err
	}
	log.Printf("connect local success, addr: %s\n", localAddr)
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		return err
	}
	log.Printf("connect remote success, addr: %s\n", localAddr)

	go c.joinConnect(localConn, remoteConn)
	go c.joinConnect(remoteConn, localConn)

	return nil
}

func (c *Client) joinConnect(a, b net.Conn) {
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
