package server

type Config struct {
	Port int
}

type ServerOptions func(*Config)

func NewConfig(port int, opts ...ServerOptions) *Config {
	res := &Config{
		Port: port,
	}
	for _, opt := range opts {
		opt(res)
	}

	return res
}
