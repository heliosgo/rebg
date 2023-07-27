package client

import "github.com/BurntSushi/toml"

type Config struct {
	Core  Core
	Items map[string]Item
}

type Core struct {
	RemoteHost string
	RemotePort int
}

type Item struct {
	Type       string
	LocalHost  string
	LocalPort  int
	RemotePort int
}

func NewConfig(file string) (Config, error) {
	var res Config
	_, err := toml.DecodeFile(file, &res)

	return res, err
}
