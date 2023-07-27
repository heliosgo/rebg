package api

type MessageType string

const (
	MessageTypeRegister  MessageType = "register"
	MessageTypeHeartBeat MessageType = "heart_beat"
	MessageTypeConnect   MessageType = "connect"

	MessageDataOK = "ok"
)

type ProtocolType string

const (
	ProtocolTypeTCP ProtocolType = "tcp"
)

type Message struct {
	Type MessageType `json:"type"`
	Data any         `json:"data"`
}

type MessageRegister struct {
	M map[string]MessageRegisterItem `json:"m"`
}

type MessageRegisterItem struct {
	Key        string       `json:"key"`
	Type       ProtocolType `json:"type"`
	RemotePort int          `json:"remote_port"`
}

type MessageConnect struct {
	LocalKey string `json:"local_key"`
}
