package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/controller"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/plugin/canopy"
	"github.com/canopy-network/canopy/store"
	"github.com/spf13/cobra"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var rootCmd = &cobra.Command{
	Use:     "canopy",
	Short:   "the canopy blockchain software",
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

// Start() is the entrypoint of the application
func Start() {
	// create a new database object from the config
	db, err := store.New(config, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	// register all plugins as active
	RegisterAllPlugins(config, validatorKey, db, l)
	// create a new instance of the application
	app, err := controller.New(config, validatorKey, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	// start the application
	app.Start()
	// start the rpc
	rpc.StartRPC(app, config, l)
	// block until a kill signal is received
	waitForKill()
	// gracefully stop the app
	app.Stop()
	// exit
	os.Exit(0)

}

// RegisterAllPlugins() registers plugins with the controller - creating consensus instances for each
func RegisterAllPlugins(c lib.Config, valKey crypto.PrivateKeyI, db lib.StoreI, l lib.LoggerI) {
	// register a new Canopy plugin
	if err := canopy.RegisterNew(c, lib.CanopyCommitteeId, valKey, db, l); err != nil {
		l.Fatal(err.Error())
	}
}

// waitForKill() blocks until a kill signal is received
func waitForKill() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGABRT)
	// block until kill signal is received
	s := <-stop
	l.Infof("Exit command %s received", s)
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
		blsPrivateKey, _ := crypto.NewBLS12381PrivateKey()
		log.Infof("Creating %s file", lib.ValKeyPath)
		if err = crypto.PrivateKeyToFile(blsPrivateKey, privateValKeyPath); err != nil {
			panic(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDirPath, lib.ProposalsFilePath)); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.ProposalsFilePath)
		proposals := make(types.GovProposals)
		a, _ := lib.NewAny(&lib.StringWrapper{Value: "example"})
		if err = proposals.Add(&types.MessageChangeParameter{
			ParameterSpace: types.ParamSpaceCons + "|" + types.ParamSpaceFee + "|" + types.ParamSpaceVal + "|" + types.ParamSpaceGov,
			ParameterKey:   types.ParamProtocolVersion,
			ParameterValue: a,
			StartHeight:    1,
			EndHeight:      1000000,
			Signer:         []byte(strings.Repeat("F", crypto.HashSize*2)),
		}, true); err != nil {
			panic(err)
		}
		if err = proposals.SaveToFile(dataDirPath); err != nil {
			panic(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDirPath, lib.PollsFilePath)); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.PollsFilePath)
		polls := types.ActivePolls{
			Polls:    map[string]map[string]bool{},
			PollMeta: map[string]*types.StartPoll{},
		}
		if err = polls.SaveToFile(dataDirPath); err != nil {
			panic(err)
		}
	}
	privateValKey, err := crypto.NewBLS12381PrivateKeyFromFile(privateValKeyPath)
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
		Time:     uint64(time.Now().UnixMicro()),
		Accounts: []*types.Account{{Address: addr.Bytes(), Amount: 1000000}},
		Validators: []*types.Validator{{
			Address:      addr.Bytes(),
			PublicKey:    consPubKey.Bytes(),
			Committees:   []uint64{lib.CanopyCommitteeId},
			NetAddress:   "http://localhost:9000",
			StakedAmount: 1000000000000,
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
