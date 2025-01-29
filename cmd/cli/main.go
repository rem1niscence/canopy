package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/controller"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/fsm/types"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/canopy-network/canopy/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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
	sm, err := fsm.New(config, db, l)
	if err != nil {
		l.Fatal(err.Error())
	}
	// create a new instance of the application
	app, err := controller.New(sm, config, validatorKey, l)
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

// waitForKill() blocks until a kill signal is received
func waitForKill() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGABRT)
	// block until kill signal is received
	s := <-stop
	l.Infof("Exit command %s received", s)
}

// InitializeDataDirectory() populates the data directory with configuration and data files if missing
func InitializeDataDirectory(dataDirPath string, log lib.LoggerI) (c lib.Config, privateValKey crypto.PrivateKeyI) {
	// make the data dir if missing
	if err := os.MkdirAll(dataDirPath, os.ModePerm); err != nil {
		log.Fatal(err.Error())
	}
	// make the config.json file if missing
	configFilePath := filepath.Join(dataDirPath, lib.ConfigFilePath)
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.ConfigFilePath)
		if err = lib.DefaultConfig().WriteToFile(configFilePath); err != nil {
			log.Fatal(err.Error())
		}
	}
	// make the private key file if missing
	privateValKeyPath := filepath.Join(dataDirPath, lib.ValKeyPath)
	if _, err := os.Stat(privateValKeyPath); errors.Is(err, os.ErrNotExist) {
		blsPrivateKey, _ := crypto.NewBLS12381PrivateKey()
		log.Infof("Creating %s file", lib.ValKeyPath)
		if err = crypto.PrivateKeyToFile(blsPrivateKey, privateValKeyPath); err != nil {
			log.Fatal(err.Error())
		}
		// get the password from the user
		log.Infof("Enter password for your new private key:")
		password, e := term.ReadPassword(int(os.Stdin.Fd()))
		if e != nil {
			log.Fatal(e.Error())
		}
		// get nickname from the user
		log.Infof("Enter nickname for your new private key:")
		var nickname string
		_, e = fmt.Scanln(&nickname)
		if e != nil {
			log.Fatal(e.Error())
		}
		// load the keystore from file
		k, e := crypto.NewKeystoreFromFile(dataDirPath)
		if e != nil {
			log.Fatal(e.Error())
		}
		// import the validator key
		address, e := k.ImportRaw(blsPrivateKey.Bytes(), crypto.ImportRawOpts{
			Password: string(password),
			Nickname: string(nickname),
		})
		if e != nil {
			log.Fatal(e.Error())
		}
		// save keystore to the file
		if e = k.SaveToFile(dataDirPath); e != nil {
			log.Fatal(e.Error())
		}
		log.Infof("Imported validator key %s to keystore", address)
	}
	// make the proposals.json file if missing
	if _, err := os.Stat(filepath.Join(dataDirPath, lib.ProposalsFilePath)); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.ProposalsFilePath)
		// create an example proposal
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
			log.Fatal(err.Error())
		}
		if err = proposals.SaveToFile(dataDirPath); err != nil {
			log.Fatal(err.Error())
		}
	}
	// load the private key object
	privateValKey, err := crypto.NewBLS12381PrivateKeyFromFile(privateValKeyPath)
	if err != nil {
		log.Fatal(err.Error())
	}
	// make the poll.json file if missing
	if _, err = os.Stat(filepath.Join(dataDirPath, lib.PollsFilePath)); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.PollsFilePath)
		// create an example poll
		examplePollHash := crypto.HashString([]byte("example"))
		polls := &types.ActivePolls{
			Polls: map[string]map[string]bool{
				examplePollHash: {privateValKey.PublicKey().Address().String(): true},
			},
			PollMeta: map[string]*types.StartPoll{
				examplePollHash: {
					StartPoll: examplePollHash,
					Url:       "https://forum.cnpy.network/something",
					EndHeight: 1000000000000,
				},
			},
		}
		if err = polls.SaveToFile(dataDirPath); err != nil {
			log.Fatal(err.Error())
		}
	}
	// create the genesis file if missing
	genesisFilePath := filepath.Join(dataDirPath, lib.GenesisFilePath)
	if _, err = os.Stat(genesisFilePath); errors.Is(err, os.ErrNotExist) {
		log.Infof("Creating %s file", lib.GenesisFilePath)
		WriteDefaultGenesisFile(privateValKey, genesisFilePath)
	}
	// load the config object
	c, err = lib.NewConfigFromFile(configFilePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	// set the data-directory
	c.DataDirPath = dataDirPath
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
			NetAddress:   "tcp://localhost",
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
