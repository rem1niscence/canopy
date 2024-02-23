package main

import (
	"github.com/ginchuco/ginchu/types"
)

func main() {
	log := types.NewLogger(types.LoggerConfig{
		Level: types.DebugLevel,
	})
	log.Debugf("hello world")
}
