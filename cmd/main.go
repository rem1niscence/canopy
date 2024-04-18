package main

import (
	"github.com/ginchuco/ginchu/consensus"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/spf13/cobra"
	"log"
)

//func main() {
//	log := types.NewLogger(types.LoggerConfig{
//		Level: types.DebugLevel,
//	})
//	for range time.Tick(time.Second) {
//		log.Info("This is a program to test docker")
//	}
//}

var (
	rootCmd = &cobra.Command{Use: "ginchu", Short: "ginchu is a generic blockchain implementation"}
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the blockchain daemon",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func Start() {
	config, logger := lib.DefaultConfig(), lib.NewDefaultLogger()
	pk, _ := crypto.NewPrivateKey()
	cs, err := consensus.New(pk, config, logger)
	if err != nil {
		logger.Fatal(err.Error())
	}
}
