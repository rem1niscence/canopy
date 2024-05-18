package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	RPCConfig
	StateMachineConfig
	StoreConfig
	P2PConfig
	ConsensusConfig
	MempoolConfig
}

func DefaultConfig() Config {
	return Config{
		RPCConfig:          DefaultRPCConfig(),
		StateMachineConfig: DefaultStateMachineConfig(),
		StoreConfig:        DefaultStoreConfig(),
		P2PConfig:          DefaultP2PConfig(),
		ConsensusConfig:    DefaultConsensusConfig(),
		MempoolConfig:      DefaultMempoolConfig(),
	}
}

type RPCConfig struct {
	Port     string
	TimeoutS int
}

func DefaultRPCConfig() RPCConfig {
	return RPCConfig{
		Port:     "6001",
		TimeoutS: 3,
	}
}

type StateMachineConfig struct{}

func DefaultStateMachineConfig() StateMachineConfig {
	return StateMachineConfig{}
}

type ConsensusConfig struct {
	ProtocolVersion         int
	NetworkID               uint32
	ElectionTimeoutMS       int
	ElectionVoteTimeoutMS   int
	ProposeTimeoutMS        int
	ProposeVoteTimeoutMS    int
	PrecommitTimeoutMS      int
	PrecommitVoteTimeoutMS  int
	CommitTimeoutMS         int
	CommitProcessMS         int // majority of block time
	RoundInterruptTimeoutMS int
}

func (c ConsensusConfig) BlockTimeMS() int {
	return c.ElectionTimeoutMS + c.ElectionVoteTimeoutMS + c.ProposeTimeoutMS + c.ProposeVoteTimeoutMS +
		c.PrecommitTimeoutMS + c.PrecommitVoteTimeoutMS + c.CommitTimeoutMS + c.CommitProcessMS
}

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
		ProtocolVersion:         1,
		NetworkID:               1,
	}
}

type P2PConfig struct {
	ListenAddress   string   // listen for incoming connection
	ExternalAddress string   // advertise for external dialing
	MaxInbound      int      // max inbound peers
	MaxOutbound     int      // max outbound peers
	TrustedPeerIDs  []string // trusted public keys
	DialPeers       []string // peers to consistently dial (format pubkey@ip:port)
	BannedPeerIDs   []string // banned public keys
	BannedIPs       []string // banned IPs
}

func DefaultP2PConfig() P2PConfig {
	return P2PConfig{
		ListenAddress:   "0.0.0.0:3000",
		ExternalAddress: "",
		MaxInbound:      21,
		MaxOutbound:     7,
	}
}

type StoreConfig struct {
	DataDirPath string
	DBName      string
	InMemory    bool
}

func DefaultDataDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ginchu")
}

func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		DataDirPath: DefaultDataDirPath(),
		DBName:      "ginchu",
		InMemory:    false,
	}
}

func (c Config) WriteToFile(filepath string) error {
	configBz, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, configBz, os.ModePerm)
}

func NewConfigFromFile(filepath string) (Config, error) {
	bz, err := os.ReadFile(filepath)
	if err != nil {
		return Config{}, err
	}
	c := new(Config)
	if err = json.Unmarshal(bz, c); err != nil {
		return Config{}, err
	}
	return *c, nil
}

const (
	ConfigFilePath  = "config.json"
	ValKeyPath      = "validator_key.json"
	NodeKeyPath     = "node_key.json"
	GenesisFilePath = "genesis.json"
)
