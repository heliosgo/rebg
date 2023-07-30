package client

import (
	"rebg/api"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Core  Core
	Items map[string]Item
}

type Core struct {
	RemoteHost string `toml:"remote_host"`
	RemotePort int    `toml:"remote_port"`
}

type Item struct {
	Type       api.ProtocolType `toml:"type"`
	LocalHost  string           `toml:"local_host"`
	LocalPort  int              `toml:"local_port"`
	RemotePort int              `toml:"remote_port"`
}

func NewConfig(file string) (Config, error) {
	var res Config
	_, err := toml.DecodeFile(file, &res)

	return res, err
}
