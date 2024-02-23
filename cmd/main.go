package main

import (
	"github.com/ginchuco/ginchu/types"
	"time"
)

func main() {
	log := types.NewLogger(types.LoggerConfig{
		Level: types.DebugLevel,
	})
	for range time.Tick(time.Second) {
		log.Info("This is a program to test docker")
	}
}
