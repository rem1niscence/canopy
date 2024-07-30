package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/ginchuco/ginchu/app"
	"github.com/ginchuco/ginchu/cmd/rpc"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/store"
	"github.com/spf13/cobra"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var rootCmd = &cobra.Command{
	Use:     "ginchu",
	Short:   "ginchu is a generic blockchain implementation",
	Version: rpc.SoftwareVersion,
}

var (
	client, config, l     = &rpc.Client{}, lib.Config{}, lib.LoggerI(nil)
	dataDir, validatorKey = "", crypto.PrivateKeyI(nil)
)

func init() {
	flag.Parse()
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(adminCmd)
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", lib.DefaultDataDirPath(), "custom data directory location")

	config, validatorKey = InitializeDataDirectory(dataDir, lib.NewDefaultLogger())
	l = lib.NewLogger(lib.LoggerConfig{Level: config.GetLogLevel()})
	client = rpc.NewClient(config.RPCUrl, config.RPCPort, config.AdminPort)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the blockchain software",
	Run: func(cmd *cobra.Command, args []string) {
		Start()
	},
}

func Start() {
	db, err := store.New(config, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	app, err := app.New(config, validatorKey, db, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	app.Start()
	rpc.StartRPC(app, config, l)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGABRT)
	s := <-stop
	app.Stop()
	l.Infof("Exit command %s received", s)
	os.Exit(0)

}

func InitializeDataDirectory(dataDirPath string, log lib.LoggerI) (c lib.Config, privateValKey crypto.PrivateKeyI) {
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
	proposalsFilePath := filepath.Join(dataDirPath, lib.ProposalsFilePath)
	if _, err := os.Stat(proposalsFilePath); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.ProposalsFilePath)
		proposals := make(types.Proposals)
		a, _ := lib.NewAny(&lib.StringWrapper{Value: "example"})
		if err = proposals.Add(&types.MessageChangeParameter{
			ParameterSpace: types.ParamSpaceCons + "|" + types.ParamSpaceFee + "|" + types.ParamSpaceVal + "|" + types.ParamSpaceGov,
			ParameterKey:   types.ParamProtocolVersion,
			ParameterValue: a,
			StartHeight:    1,
			EndHeight:      1000000,
			Signer:         lib.MaxHash,
		}, true); err != nil {
			panic(err)
		}
		if err = proposals.SaveToFile(dataDirPath); err != nil {
			panic(err)
		}
	}
	privateValKey, err := crypto.NewBLSPrivateKeyFromFile(privateValKeyPath)
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
		WriteDefaultGenesisFile(privateValKey, genesisFilePath)
	}
	return
}

func WriteDefaultGenesisFile(validatorPrivateKey crypto.PrivateKeyI, genesisFilePath string) {
	consPubKey := validatorPrivateKey.PublicKey()
	addr := consPubKey.Address()
	j := &types.GenesisState{
		Time:     timestamppb.Now(),
		Pools:    []*types.Pool{{Id: types.PoolID_DAO}, {Id: types.PoolID_FeeCollector}},
		Accounts: []*types.Account{{Address: addr.Bytes(), Amount: 1000000}},
		Validators: []*types.Validator{{
			Address:      addr.Bytes(),
			PublicKey:    consPubKey.Bytes(),
			NetAddress:   "http://localhost:9000",
			StakedAmount: 1000000000000000000,
			Output:       addr.Bytes(),
		}},
		Params: types.DefaultParams(),
	}
	bz, _ := json.MarshalIndent(j, "", "  ")
	if err := os.WriteFile(genesisFilePath, bz, 0777); err != nil {
		panic(err)
	}
}

func writeToConsole(a any, err error) {
	if err != nil {
		l.Fatal(err.Error())
	}
	switch a.(type) {
	case int, uint32, uint64:
		p := message.NewPrinter(language.English)
		if _, err := p.Printf("%d\n", a); err != nil {
			l.Fatal(err.Error())
		}
	case string, *string:
		fmt.Println(a)
	default:
		s, err := lib.MarshalJSONIndentString(a)
		if err != nil {
			l.Fatal(err.Error())
		}
		fmt.Println(s)
	}
}
