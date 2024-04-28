package main

import (
	"encoding/json"
	"errors"
	"github.com/ginchuco/ginchu/consensus"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/store"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var (
	rootCmd = &cobra.Command{Use: "ginchu", Short: "ginchu is a generic blockchain implementation"}
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the blockchain daemon",
	Run: func(cmd *cobra.Command, args []string) {
		Start()
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
	l := lib.NewDefaultLogger()
	c, valKey, nodeKey, db := InitializeDataDirectory("", l)
	app, err := consensus.New(c, valKey, nodeKey, db, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	app.Start()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGABRT)
	s := <-stop
	app.Stop()
	l.Infof("Exit command %s received", s)
	os.Exit(0)

}

func InitializeDataDirectory(dataDirPath string, log lib.LoggerI) (c lib.Config, privateValKey, privateNodeKey crypto.PrivateKeyI, db lib.StoreI) {
	if dataDirPath == "" {
		dataDirPath = lib.DefaultDataDirPath()
	}
	log.Infof("Reading data directory at %s", dataDirPath)
	if err := os.MkdirAll(dataDirPath, os.ModePerm); err != nil {
		panic(err)
	}
	configFilePath := filepath.Join(dataDirPath, lib.ConfigFilePath)
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.ConfigFilePath)
		if err = lib.DefaultConfig().WriteToFile(configFilePath); err != nil {
			panic(err)
		}
	}
	privateValKeyPath := filepath.Join(dataDirPath, lib.ValKeyPath)
	if _, err := os.Stat(privateValKeyPath); errors.Is(err, os.ErrNotExist) {
		blsPrivateKey, _ := crypto.NewBLSPrivateKey()
		log.Infof("Creating %s file", lib.ValKeyPath)
		if err = crypto.PrivateKeyToFile(blsPrivateKey, privateValKeyPath); err != nil {
			panic(err)
		}
	}
	privateNodeKeyPath := filepath.Join(dataDirPath, lib.NodeKeyPath)
	if _, err := os.Stat(privateNodeKeyPath); errors.Is(err, os.ErrNotExist) {
		ed25519PrivateKey, _ := crypto.NewPrivateKey()
		log.Infof("Creating %s file", lib.NodeKeyPath)
		if err = crypto.PrivateKeyToFile(ed25519PrivateKey, privateNodeKeyPath); err != nil {
			panic(err)
		}
	}
	privateValKey, err := crypto.NewBLSPrivateKeyFromFile(privateValKeyPath)
	if err != nil {
		panic(err)
	}
	privateNodeKey, err = crypto.NewED25519PrivateKeyFromFile(privateNodeKeyPath)
	if err != nil {
		panic(err)
	}
	c, err = lib.NewConfigFromFile(configFilePath)
	if err != nil {
		panic(err)
	}
	c.DataDirPath = dataDirPath
	genesisFilePath := filepath.Join(dataDirPath, lib.GenesisFilePath)
	if _, err = os.Stat(genesisFilePath); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.GenesisFilePath)
		WriteDefaultGenesisFile(privateValKey, privateNodeKey, genesisFilePath)
	}
	db, err = store.New(c, log)
	if err != nil {
		panic(err)
	}
	return
}

func WriteDefaultGenesisFile(validatorPrivateKey, nodePrivateKey crypto.PrivateKeyI, genesisFilePath string) {
	pubKey := nodePrivateKey.PublicKey()
	address, consPubKey := pubKey.Address(), validatorPrivateKey.PublicKey()
	j := &types.GenesisState{
		Time:     timestamppb.Now(),
		Pools:    []*types.Pool{{Id: types.PoolID_DAO}, {Id: types.PoolID_FeeCollector}},
		Accounts: []*types.Account{{Address: address.Bytes(), Amount: 1000000}},
		Validators: []*types.Validator{{
			Address:      consPubKey.Address().Bytes(),
			PublicKey:    consPubKey.Bytes(),
			NetAddress:   "http://localhost:3000",
			StakedAmount: 1000000,
			Output:       address.Bytes(),
		}},
		Params: types.DefaultParams(),
	}
	bz, _ := json.MarshalIndent(j, "", "  ")
	if err := os.WriteFile(genesisFilePath, bz, 0777); err != nil {
		panic(err)
	}
}
