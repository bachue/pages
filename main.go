package main

import (
	"log"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"github.com/bachue/pages/sshd"
)

func main() {
	// TODO: Usage
	err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	logger, err := log_driver.New(&config.Current.Log)
	if err != nil {
		log.Fatal(err)
	}
	sshdServer, err := sshd.NewServer(&config.Current.Sshd, logger)
	if err != nil {
		log.Fatal(err)
	}
	err = sshdServer.Start()
	if err != nil {
		log.Fatal(err)
	}
}
