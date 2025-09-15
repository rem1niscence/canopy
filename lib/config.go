package lib

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/units"
)

/* This file implements logic for 'user controlled' global configurations of each module of the node */

const (
	// GLOBAL CONSTANTS
	UnknownChainId         = uint64(0)            // the default 'unknown' chain id
	CanopyChainId          = uint64(1)            // NOTE: to not break nested-chain recursion, this should not be used except for 'default config/genesis' developer setups
	DAOPoolID              = 2*math.MaxUint16 + 1 // must be above the MaxUint16 * 2 to ensure no 'overlap' with 'chainId + EscrowAddend'
	CanopyMainnetNetworkId = 1                    // the identifier of the 'mainnet' of Canopy
)

const (
	// FILE NAMES in the 'data directory'
	ConfigFilePath    = "config.json"        // the file path for the node configuration
	ValKeyPath        = "validator_key.json" // the file path for the node's private key
	GenesisFilePath   = "genesis.json"       // the file path for the genesis (first block)
	ProposalsFilePath = "proposals.json"     // the file path for governance proposal voting configuration
	PollsFilePath     = "polls.json"         // the file path for governance 'straw' polling voting and tracking
)

// Config is the structure of the user configuration options for a Canopy node
type Config struct {
	MainConfig         // main options spanning over all modules
	LoggerConfig       // logger options
	RPCConfig          // rpc API options
	StateMachineConfig // FSM options
	StoreConfig        // persistence options
	P2PConfig          // peer-to-peer options
	ConsensusConfig    // bft options
	MempoolConfig      // mempool options
	MetricsConfig      // telemetry options
}

// DefaultConfig() returns a Config with developer set options
func DefaultConfig() Config {
	return Config{
		MainConfig:         DefaultMainConfig(),
		RPCConfig:          DefaultRPCConfig(),
		StateMachineConfig: DefaultStateMachineConfig(),
		StoreConfig:        DefaultStoreConfig(),
		P2PConfig:          DefaultP2PConfig(),
		ConsensusConfig:    DefaultConsensusConfig(),
		MempoolConfig:      DefaultMempoolConfig(),
		MetricsConfig:      DefaultMetricsConfig(),
	}
}

// MAIN CONFIG BELOW

type MainConfig struct {
	LogLevel   string      `json:"logLevel"`   // any level includes the levels above it: debug < info < warning < error
	ChainId    uint64      `json:"chainId"`    // the identifier of this particular chain within a single 'network id'
	SleepUntil uint64      `json:"sleepUntil"` // allows coordinated 'wake-ups' for genesis or chain halt events
	RootChain  []RootChain `json:"rootChain"`  // a list of the root chain(s) a node could connect to as dictated by the governance parameter 'RootChainId'
	RunVDF     bool        `json:"runVDF"`     // whether the node should run a Verifiable Delay Function to help secure the network against Long-Range-Attacks
	Headless   bool        `json:"headless"`   // turn off the web wallet and block explorer 'web' front ends
	AutoUpdate bool        `json:"autoUpdate"` // check for new versions of software each X time
}

// DefaultMainConfig() sets log level to 'info'
func DefaultMainConfig() MainConfig {
	return MainConfig{
		LogLevel: "info", // everything but debug is the default
		RootChain: []RootChain{
			{
				ChainId: CanopyChainId,            // RootChainId is chain id 1
				Url:     "http://localhost:50002", // RooChainURL points to self
			},
		},
		RunVDF:     true,          // run the VDF by default
		ChainId:    CanopyChainId, // default chain url is 1
		Headless:   false,         // serve the web wallet and block explorer by default
		AutoUpdate: true,          // set it as default while in inmature state
	}
}

// GetLogLevel() parses the log string in the config file into a LogLevel Enum
func (m *MainConfig) GetLogLevel() int32 {
	switch {
	case strings.Contains(strings.ToLower(m.LogLevel), "deb"):
		return DebugLevel
	case strings.Contains(strings.ToLower(m.LogLevel), "inf"):
		return InfoLevel
	case strings.Contains(strings.ToLower(m.LogLevel), "war"):
		return WarnLevel
	case strings.Contains(strings.ToLower(m.LogLevel), "err"):
		return ErrorLevel
	default:
		return DebugLevel
	}
}

// RPC CONFIG BELOW

type RPCConfig struct {
	WalletPort   string `json:"walletPort"`   // the port where the web wallet is hosted
	ExplorerPort string `json:"explorerPort"` // the port where the block explorer is hosted
	RPCPort      string `json:"rpcPort"`      // the port where the rpc server is hosted
	AdminPort    string `json:"adminPort"`    // the port where the admin rpc server is hosted
	RPCUrl       string `json:"rpcURL"`       // the url where the rpc server is hosted
	AdminRPCUrl  string `json:"adminRPCUrl"`  // the url where the admin rpc server is hosted
	TimeoutS     int    `json:"timeoutS"`     // the rpc request timeout in seconds
}

// RootChain defines a rpc url to a possible 'root chain' which is used if the governance parameter RootChainId == ChainId
type RootChain struct {
	ChainId uint64 `json:"chainId"` // used if the governance parameter RootChainId == ChainId
	Url     string `json:"url"`     // the url to the 'root chain' rpc
}

// DefaultRPCConfig() sets rpc url to localhost and sets wallet, explorer, rpc, and admin ports from [50000-50003]
func DefaultRPCConfig() RPCConfig {
	return RPCConfig{
		WalletPort:   "50000",                  // find the wallet on localhost:50000
		ExplorerPort: "50001",                  // find the explorer on localhost:50001
		RPCPort:      "50002",                  // the rpc is served on localhost:50002
		AdminPort:    "50003",                  // the admin rpc is served on localhost:50003
		RPCUrl:       "http://localhost:50002", // use a local rpc by default
		AdminRPCUrl:  "http://localhost:50003", // use a local admin rpc by default
		TimeoutS:     3,                        // the rpc timeout is 3 seconds
	}
}

// STATE MACHINE CONFIG BELOW

// StateMachineConfig is an empty placeholder
type StateMachineConfig struct{}

// DefaultStateMachineConfig returns an empty object
func DefaultStateMachineConfig() StateMachineConfig { return StateMachineConfig{} }

// CONSENSUS CONFIG BELOW

// ConsensusConfig defines the consensus phase timeouts for bft synchronicity
// NOTES:
// - BlockTime = ElectionTimeout + ElectionVoteTimeout + ProposeTimeout + ProposeVoteTimeout + PrecommitTimeout + PrecommitVoteTimeout + CommitTimeout + CommitProcess
// - async faults may lead to extended block time
// - social consensus dictates BlockTime for the protocol - being oo fast or too slow can lead to Non-Signing and Consensus failures
type ConsensusConfig struct {
	NewHeightTimeoutMs      int `json:"newHeightTimeoutMS"`      // how long (in milliseconds) the replica sleeps before moving to the ELECTION phase
	ElectionTimeoutMS       int `json:"electionTimeoutMS"`       // minus VRF creation time (if Candidate), is how long (in milliseconds) the replica sleeps before moving to ELECTION-VOTE phase
	ElectionVoteTimeoutMS   int `json:"electionVoteTimeoutMS"`   // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to PROPOSE phase
	ProposeTimeoutMS        int `json:"proposeTimeoutMS"`        // minus Proposal creation time (if Leader), is how long (in milliseconds) the replica sleeps before moving to PROPOSE-VOTE phase
	ProposeVoteTimeoutMS    int `json:"proposeVoteTimeoutMS"`    // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to PRECOMMIT phase
	PrecommitTimeoutMS      int `json:"precommitTimeoutMS"`      // minus Proposal-QC aggregation time (if Leader), how long (in milliseconds) the replica sleeps before moving to the PRECOMMIT-VOTE phase
	PrecommitVoteTimeoutMS  int `json:"precommitVoteTimeoutMS"`  // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to COMMIT phase
	CommitTimeoutMS         int `json:"commitTimeoutMS"`         // minus Precommit-QC aggregation time (if Leader), how long (in milliseconds) the replica sleeps before moving to the COMMIT-PROCESS phase
	RoundInterruptTimeoutMS int `json:"roundInterruptTimeoutMS"` // minus gossiping current Round time, how long (in milliseconds) the replica sleeps before moving to PACEMAKER phase
}

// DefaultConsensusConfig() configures the block time
func DefaultConsensusConfig() ConsensusConfig {
	return ConsensusConfig{
		NewHeightTimeoutMs:     4500, // 4.5 seconds
		ElectionTimeoutMS:      1500, // 1.5 seconds
		ElectionVoteTimeoutMS:  1500, // 1.5 seconds
		ProposeTimeoutMS:       2500, // 2.5 seconds
		ProposeVoteTimeoutMS:   4000, // 4 seconds
		PrecommitTimeoutMS:     2000, // 2 seconds
		PrecommitVoteTimeoutMS: 2000, // 2 seconds
		CommitTimeoutMS:        2000, // 2 seconds
	}
}

// BlockTimeMS() returns the expected block time in milliseconds
func (c *ConsensusConfig) BlockTimeMS() int {
	return c.NewHeightTimeoutMs +
		c.ElectionTimeoutMS +
		c.ElectionVoteTimeoutMS +
		c.ProposeTimeoutMS +
		c.ProposeVoteTimeoutMS +
		c.PrecommitTimeoutMS +
		c.PrecommitVoteTimeoutMS +
		c.CommitTimeoutMS
}

// P2P CONFIG BELOW

// P2PConfig defines peering compatibility and limits as well as actions on specific peering IPs / IDs
type P2PConfig struct {
	NetworkID           uint64   `json:"networkID"`           // the ID for the peering network
	ListenAddress       string   `json:"listenAddress"`       // listen for incoming connection
	ExternalAddress     string   `json:"externalAddress"`     // advertise for external dialing
	MaxInbound          int      `json:"maxInbound"`          // max inbound peers
	MaxOutbound         int      `json:"maxOutbound"`         // max outbound peers
	TrustedPeerIDs      []string `json:"trustedPeerIDs"`      // trusted public keys
	DialPeers           []string `json:"dialPeers"`           // peers to consistently dial until expo-backoff fails (format pubkey@ip:port)
	BannedPeerIDs       []string `json:"bannedPeersIDs"`      // banned public keys
	BannedIPs           []string `json:"bannedIPs"`           // banned IPs
	MinimumPeersToStart int      `json:"minimumPeersToStart"` // the minimum connections required to start consensus
}

func DefaultP2PConfig() P2PConfig {
	return P2PConfig{
		NetworkID:           CanopyMainnetNetworkId,
		ListenAddress:       "0.0.0.0:9001", // default TCP address is 9001 for chain 1 (9002 for chain 2 etc.)
		ExternalAddress:     "",             // should be populated by the user
		MaxInbound:          21,             // inbounds should be close to 3x greater than outbounds
		MaxOutbound:         7,              // to ensure 'new joiners' have slots to take
		MinimumPeersToStart: 0,              // requires no peers to start consensus by default (suitable for 1 node network)
	}
}

// STORE CONFIG BELOW

// StoreConfig is user configurations for the key value database
type StoreConfig struct {
	DataDirPath          string `json:"dataDirPath"`          // path of the designated folder where the application stores its data
	DBName               string `json:"dbName"`               // name of the database
	IndexByAccount       bool   `json:"indexByAccount"`       // index transactions by account
	InMemory             bool   `json:"inMemory"`             // non-disk database, only for testing
	CleanupBlockInterval uint64 `json:"cleanupBlockInterval"` // interval for cleaning up stale data, in blocks
}

// DefaultDataDirPath() is $USERHOME/.canopy
func DefaultDataDirPath() string {
	// get the user home
	home, err := os.UserHomeDir()
	// home, err := os.Getwd()
	// if unable to get the user home
	if err != nil {
		// fatal error
		panic(err)
	}
	// exit with full default data directory path
	// return filepath.Join(home, "canopy_2")
	return filepath.Join(home, ".canopy")
}

// DefaultStoreConfig() returns the developer recommended store configuration
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		DataDirPath:          DefaultDataDirPath(),         // use the default data dir path
		DBName:               "canopy",                     // 'canopy' database name
		IndexByAccount:       true,                         // index transactions by account
		InMemory:             false,                        // persist to disk, not memory
		CleanupBlockInterval: uint64(rand.Intn(101) + 100), // clean every 100-200 blocks (random)
	}
}

// MEMPOOL CONFIG BELOW

// MempoolConfig is the user configuration of the unconfirmed transaction pool
type MempoolConfig struct {
	MaxTotalBytes              uint64 `json:"maxTotalBytes"`              // maximum collective bytes in the pool
	MaxTransactionCount        uint32 `json:"maxTransactionCount"`        // max number of Transactions
	IndividualMaxTxSize        uint32 `json:"individualMaxTxSize"`        // max bytes of a single Transaction
	DropPercentage             int    `json:"dropPercentage"`             // percentage that is dropped from the bottom of the queue if limits are reached
	LazyMempoolCheckFrequencyS int    `json:"lazyMempoolCheckFrequencyS"` // how often the mempool is checked for new transactions besides the mandatory (after Commit) (0) for none
}

// DefaultMempoolConfig() returns the developer created Mempool options
func DefaultMempoolConfig() MempoolConfig {
	return MempoolConfig{
		MaxTotalBytes:              uint64(10 * units.MB),      // 10 MB max size
		MaxTransactionCount:        5000,                       // 5000 max transactions
		IndividualMaxTxSize:        uint32(4 * units.Kilobyte), // 4 KB max individual tx size
		DropPercentage:             35,                         // drop 35% if limits are reached
		LazyMempoolCheckFrequencyS: 1,                          // check every 1 second
	}
}

// MetricsConfig represents the configuration for the metrics server
type MetricsConfig struct {
	MetricsEnabled    bool   `json:"metricsEnabled"`    // if the metrics are enabled
	PrometheusAddress string `json:"prometheusAddress"` // the address of the server
}

// DefaultMetricsConfig() returns the default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		MetricsEnabled:    true,           // enabled by default
		PrometheusAddress: "0.0.0.0:9090", // the default prometheus address
	}
}

// WriteToFile() saves the Config object to a JSON file
func (c Config) WriteToFile(filepath string) error {
	// convert the config to indented 'pretty' json bytes
	jsonBytes, err := json.MarshalIndent(c, "", "  ")
	// if an error occurred during the conversion
	if err != nil {
		// exit with error
		return err
	}
	// write the config.json file to the data directory
	return os.WriteFile(filepath, jsonBytes, os.ModePerm)
}

// NewConfigFromFile() populates a Config object from a JSON file
func NewConfigFromFile(filepath string) (Config, error) {
	// read the file into bytes using
	fileBytes, err := os.ReadFile(filepath)
	// if an error occurred
	if err != nil {
		// exit with error
		return Config{}, err
	}
	// define the default config to fill in any blanks in the file
	c := DefaultConfig()
	// populate the default config with the file bytes
	if err = json.Unmarshal(fileBytes, &c); err != nil {
		// exit with error
		return Config{}, err
	}
	// exit
	return c, nil
}
