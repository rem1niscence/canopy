// nolint:all
package main

import (
	"github.com/ginchuco/ginchu/cmd/rpc"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"math/rand"
	"sync"
	"time"
)

const (
	localhost      = "http://localhost"
	configFileName = "fuzzer.json"

	SendMsgName      = "send"
	StakeMsgName     = "stake"
	EditStakeMsgName = "edit_stake"
	UnstakeMsgName   = "unstake"
	PauseMsgName     = "pause"
	UnpauseMsgName   = "unpause"

	BadSigReason        = "bad signature"
	BadSeqReason        = "bad sequence"
	BadFeeReason        = "bad fee"
	BadMessageReason    = "bad msg"
	BadSenderReason     = "bad sender"
	BadRecReason        = "bad receiver"
	BadAmountReason     = "bad amount"
	BadNetAddrReason    = "bad net address"
	BadOutputAddrReason = "bad output address"
)

func main() {
	fuzzer := NewFuzzer()
	go fuzzer.resetDependentStateLoop()
	fuzzer.config.PercentInvalidTransactions = 0
	for range time.Tick(100 * time.Millisecond) {
		switch rand.Intn(4) {
		case 0:
			if err := fuzzer.SendTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		case 1:
			if err := fuzzer.StakeTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		case 2:
			if err := fuzzer.EditStakeTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		case 3:
			if err := fuzzer.PauseTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		case 4:
			if err := fuzzer.UnpauseTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		case 5:
			if err := fuzzer.UnstakeTransaction(); err != nil {
				fuzzer.log.Error(err.Error())
			}
		}
	}
}

type Fuzzer struct {
	log    lib.LoggerI
	config *Config
	client *rpc.Client
	state  *DependentState
}

func NewFuzzer() *Fuzzer {
	log := lib.NewDefaultLogger()
	config := DefaultConfig().FromFile(log)
	return &Fuzzer{
		log:    log,
		config: config,
		client: rpc.NewClient(config.RPCUrl, config.RPCPort, config.AdminRPCPort),
		state: &DependentState{
			RWMutex:  sync.RWMutex{},
			height:   0,
			accounts: make(map[string]*types.Account),
			vals:     make(map[string]*types.Validator),
		},
	}
}

func (f *Fuzzer) resetDependentStateLoop() {
	for range time.Tick(5 * time.Second) {
		height, err := f.client.Height()
		if err != nil {
			f.log.Error(err.Error())
			continue
		}
		f.state.RLock()
		reset := *height > f.state.height
		f.state.RUnlock()
		if reset {
			f.log.Infof("New height detected: %d, resetting dependent state", *height)
			f.state.Lock()
			f.state.Reset(*height)
			f.state.Unlock()
		}
	}
}
