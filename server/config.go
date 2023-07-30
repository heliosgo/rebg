package server

import "github.com/BurntSushi/toml"

type Config struct {
	Port int `toml:"port"`
}

type ServerOptions func(*Config)

func NewConfig(file string, opts ...ServerOptions) (Config, error) {
	var res Config
	_, err := toml.DecodeFile(file, &res)
	if err != nil {
		return res, err
	}

	for _, opt := range opts {
		opt(&res)
	}

	return res, nil
}
