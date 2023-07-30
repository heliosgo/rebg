package main

import (
	"flag"
	"fmt"
	"log"
	"rebg/client"
)

func main() {
	var file string
	flag.StringVar(&file, "f", "/etc/rebg/client.toml", "config file path")
	flag.Parse()
	fmt.Println(file)
	app, err := client.NewClient(file)
	if err != nil {
		log.Printf("init client failed, err: %v\n", err)
		return
	}

	if err := app.Start(); err != nil {
		log.Printf("something wrong, err: %v\n", err)
	}
}
