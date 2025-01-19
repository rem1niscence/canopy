package lib

import (
	"encoding/json"
	"github.com/alecthomas/units"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Config is the structure of the user configuration options for a Canopy node
type Config struct {
	MainConfig         // main options spanning over all modules
	RPCConfig          // rpc API options
	StateMachineConfig // FSM options
	StoreConfig        // persistence options
	P2PConfig          // peer-to-peer options
	ConsensusConfig    // bft options
	MempoolConfig      // mempool options
	PluginsConfig      // committee software options
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
		PluginsConfig:      DefaultPluginConfig(),
	}
}

// MAIN CONFIG BELOW

type MainConfig struct {
	LogLevel string `json:"logLevel"`
}

// DefaultMainConfig() sets log level to 'info'
func DefaultMainConfig() MainConfig { return MainConfig{"info"} }

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
	WalletPort      string // the port where the web wallet is hosted
	ExplorerPort    string // the port where the block explorer is hosted
	RPCPort         string // the port where the rpc server is hosted
	AdminPort       string // the port where the admin rpc server is hosted
	RPCUrl          string // the url without port where the rpc server is hosted
	BaseChainRPCURL string // the url of the base-chain's rpc
	BaseChainPollMS uint64 // how often to poll the base chain in milliseconds
	TimeoutS        int    // the rpc request timeout in seconds
}

// DefaultRPCConfig() sets rpc url to localhost and sets wallet, explorer, rpc, and admin ports from [50000-50003]
func DefaultRPCConfig() RPCConfig {
	return RPCConfig{
		WalletPort:      "50000",
		ExplorerPort:    "50001",
		RPCPort:         "50002",
		AdminPort:       "50003",
		RPCUrl:          "http://localhost",
		BaseChainRPCURL: "http://localhost:50002",
		BaseChainPollMS: 1000,
		TimeoutS:        3,
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
// - social consensus dictates BlockTime for the protocol - being too fast or too slow can lead to Non-Signing and Consensus failures
type ConsensusConfig struct {
	ElectionTimeoutMS       int // minus VRF creation time (if Candidate), is how long (in milliseconds) the replica sleeps before moving to ELECTION-VOTE phase
	ElectionVoteTimeoutMS   int // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to PROPOSE phase
	ProposeTimeoutMS        int // minus Proposal creation time (if Leader), is how long (in milliseconds) the replica sleeps before moving to PROPOSE-VOTE phase
	ProposeVoteTimeoutMS    int // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to PRECOMMIT phase
	PrecommitTimeoutMS      int // minus Proposal-QC aggregation time (if Leader), how long (in milliseconds) the replica sleeps before moving to the PRECOMMIT-VOTE phase
	PrecommitVoteTimeoutMS  int // minus QC validation + vote time, is how long (in milliseconds) the replica sleeps before moving to COMMIT phase
	CommitTimeoutMS         int // minus Precommit-QC aggregation time (if Leader), how long (in milliseconds) the replica sleeps before moving to the COMMIT-PROCESS phase
	CommitProcessMS         int // minus Commit Proposal time, how long (in milliseconds) the replica sleeps before 'NewHeight' (NOTE: this is the majority of the block time)
	RoundInterruptTimeoutMS int // minus gossiping current Round time, how long (in milliseconds) the replica sleeps before moving to PACEMAKER phase
}

// DefaultConsensusConfig() configures 10 minute blocks
func DefaultConsensusConfig() ConsensusConfig {
	return ConsensusConfig{
		ElectionTimeoutMS:      5000,
		ElectionVoteTimeoutMS:  2000,
		ProposeTimeoutMS:       2000,
		ProposeVoteTimeoutMS:   2000,
		PrecommitTimeoutMS:     2000,
		PrecommitVoteTimeoutMS: 2000,
		CommitTimeoutMS:        2000,
		CommitProcessMS:        5000,
		//CommitProcessMS:         583000, // 10 minute blocks - ^
		RoundInterruptTimeoutMS: 5000,
	}
}

// P2P CONFIG BELOW

// P2PConfig defines peering compatibility and limits as well as actions on specific peering IPs / IDs
type P2PConfig struct {
	NetworkID       uint64   // the ID for the peering network
	ListenAddress   string   // listen for incoming connection
	ExternalAddress string   // advertise for external dialing
	MaxInbound      int      // max inbound peers
	MaxOutbound     int      // max outbound peers
	TrustedPeerIDs  []string // trusted public keys
	DialPeers       []string // peers to consistently dial until expo-backoff fails (format pubkey@ip:port)
	BannedPeerIDs   []string // banned public keys
	BannedIPs       []string // banned IPs
}

func DefaultP2PConfig() P2PConfig {
	return P2PConfig{
		NetworkID:       CanopyMainnetNetworkId,
		ListenAddress:   "0.0.0.0:9000", // default RPC address is 9000
		ExternalAddress: "",
		MaxInbound:      21,
		MaxOutbound:     7,
	}
}

// STORE CONFIG BELOW

// StoreConfig is user configurations for the key value database
type StoreConfig struct {
	DataDirPath string // path of the designated folder where the application stores its data
	DBName      string // name of the database
	InMemory    bool   // non-disk database, only for testing
}

// DefaultDataDirPath() is $USERHOME/.canopy
func DefaultDataDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".canopy")
}

// DefaultStoreConfig() returns the developer recommended store configuration
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		DataDirPath: DefaultDataDirPath(),
		DBName:      "canopy",
		InMemory:    false,
	}
}

// MEMPOOL CONFIG BELOW

// MempoolConfig is the user configuration of the unconfirmed transaction pool
type MempoolConfig struct {
	MaxTotalBytes       uint64 // maximum collective bytes in the pool
	MaxTransactionCount uint32 // max number of Transactions
	IndividualMaxTxSize uint32 // max bytes of a single Transaction
	DropPercentage      int    // percentage that is dropped from the bottom of the queue if limits are reached
}

// DefaultMempoolConfig() returns the developer created Mempool options
func DefaultMempoolConfig() MempoolConfig {
	return MempoolConfig{
		MaxTotalBytes:       uint64(units.MB),
		IndividualMaxTxSize: uint32(4 * units.Kilobyte),
		MaxTransactionCount: 5000,
		DropPercentage:      35,
	}
}

// PLUGINS CONFIG BELOW

// PluginsConfig is a list of the 'add-on' software that dictates and enables Committee consensus
type PluginsConfig struct {
	Plugins []PluginConfig
}

// PluginConfig defines the 'add-on' software that dictates and enables Committee consensus
type PluginConfig struct {
	ID        uint64    // socially conceived numerical identifier of the satellite chain
	Name      string    // non-used (only for user clarity), human-readable identifier
	URL       string    // url where the plugin may reach the 3rd party software (ex. http://localhost:1234)
	BasicAuth BasicAuth // optional basic authentication the plugin may use to reach the 3rd party software
}

// BasicAuth is the golang abstraction of the simple user/pass http authentication scheme
type BasicAuth struct {
	Username string
	Password string
}

// DefaultPluginConfig() returns the developer created Plugins options
func DefaultPluginConfig() PluginsConfig {
	return PluginsConfig{Plugins: []PluginConfig{{
		ID:   0,
		Name: "example_name (this is not-critical an may be anything that is helpful)",
		URL:  "http://example-of-my-fullnode.fake",
		BasicAuth: BasicAuth{
			Username: "user",
			Password: "pass",
		},
	}}}
}

// WriteToFile() saves the Config object to a JSON file
func (c Config) WriteToFile(filepath string) error {
	configBz, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, configBz, os.ModePerm)
}

// NewConfigFromFile() populates a Config object from a JSON file
func NewConfigFromFile(filepath string) (Config, error) {
	bz, err := os.ReadFile(filepath)
	if err != nil {
		return Config{}, err
	}
	c := DefaultConfig()
	if err = json.Unmarshal(bz, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

const (
	ConfigFilePath         = "config.json"
	ValKeyPath             = "validator_key.json"
	GenesisFilePath        = "genesis.json"
	ProposalsFilePath      = "proposals.json"
	PollsFilePath          = "polls.json"
	UnknownCommitteeId     = uint64(0)
	CanopyCommitteeId      = uint64(1)
	DAOPoolID              = math.MaxUint32 + 1
	CanopyMainnetNetworkId = 1
)
