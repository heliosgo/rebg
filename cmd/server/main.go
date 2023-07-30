package main

import (
	"flag"
	"log"
	"rebg/server"
)

func main() {
	var file string
	flag.StringVar(&file, "f", "/etc/rebg/server.toml", "config file path")
	flag.Parse()
	app, err := server.NewServer(file)
	if err != nil {
		log.Printf("init server failed, err: %v\n", err)
		return
	}

	if err := app.Start(); err != nil {
		log.Printf("something wrong, err: %v\n", err)
	}
}
