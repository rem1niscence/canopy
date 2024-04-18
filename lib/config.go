package lib

import (
	"os"
	"path/filepath"
)

type Config struct {
	StoreConfig
	P2PConfig
	ConsensusConfig
	MempoolConfig
}

func DefaultConfig() Config {
	return Config{
		StoreConfig:     DefaultStoreConfig(),
		P2PConfig:       DefaultP2PConfig(),
		ConsensusConfig: DefaultConsensusConfig(),
		MempoolConfig:   DefaultMempoolConfig(),
	}
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

func DefaultConsensusConfig() ConsensusConfig {
	return ConsensusConfig{
		ElectionTimeoutMS:       5000,
		ElectionVoteTimeoutMS:   2000,
		ProposeTimeoutMS:        2000,
		ProposeVoteTimeoutMS:    2000,
		PrecommitTimeoutMS:      2000,
		PrecommitVoteTimeoutMS:  2000,
		CommitTimeoutMS:         2000,
		CommitProcessMS:         583000, // 10 minute blocks - ^
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
		ListenAddress:   "tcp://0.0.0.0:3000",
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

func DefaultStoreConfig() StoreConfig {
	home, _ := os.UserHomeDir()
	return StoreConfig{
		DataDirPath: filepath.Join(home, ".ginchu"),
		DBName:      "ginchu",
		InMemory:    false,
	}
}
