package main

import (
	"log"
	"sync"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/gitfuse"
	"github.com/bachue/pages/log_driver"
	"github.com/bachue/pages/sshd"
)

func main() {
	// TODO: Usage
	err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}
	logger, err := log_driver.New(&config.Current.Log)
	if err != nil {
		log.Fatalf("Failed to create logger: %s", err)
	}

	var waitgroup sync.WaitGroup
	waitgroup.Add(2)

	go func() {
		sshdServer, err := sshd.NewServer(&config.Current.Sshd, logger)
		if err != nil {
			logger.Fatalf("Failed to create SSHD server: %s", err)
		}
		err = sshdServer.Start()
		if err != nil {
			logger.Fatalf("Failed to start SSHD server: %s", err)
		}
		waitgroup.Done()
	}()

	go func() {
		gitfs, err := gitfuse.New(&config.Current.Fuse, logger)
		if err != nil {
			logger.Fatalf("Failed to start GitFS: %s", err)
		}
		gitfs.Start()
		waitgroup.Done()
	}()

	waitgroup.Wait()
}
