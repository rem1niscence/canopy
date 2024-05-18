package main

import (
	"github.com/ginchuco/ginchu/cmd/rpc"
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"sync"
	"time"
)

const (
	localhost      = "http://localhost"
	configFileName = "fuzzer.json"

	SendMsgName = "send"

	BadSigReason     = "bad signature"
	BadSeqReason     = "bad sequence"
	BadFeeReason     = "bad fee"
	BadMessageReason = "bad msg"
	BadSenderReason  = "bad sender"
	BadRecReason     = "bad receiver"
	BadAmountReason  = "bad amount"
)

func main() {
	fuzzer := NewFuzzer()
	go fuzzer.resetDependentStateLoop()
	fuzzer.config.PercentInvalidTransactions = 0
	for range time.Tick(100 * time.Millisecond) {
		if err := fuzzer.SendTransaction(); err != nil {
			fuzzer.log.Error(err.Error())
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
	config := new(Config).FromFile(log)
	return &Fuzzer{
		log:    log,
		config: config,
		client: rpc.NewClient(config.RPCUrl, config.RPCPort),
		state: &DependentState{
			RWMutex:  sync.RWMutex{},
			height:   0,
			accounts: make(map[string]*types.Account),
		},
	}
}

func (f *Fuzzer) getAccount(address crypto.AddressI) (acc *types.Account) {
	if cached, ok := f.state.GetAccount(address); ok {
		return cached
	}
	acc, err := f.client.Account(0, address.String())
	if err != nil {
		f.log.Fatal(err.Error())
	}
	return
}

func (f *Fuzzer) getFees() (p *types.FeeParams) {
	p, err := f.client.FeeParams(0)
	if err != nil {
		f.log.Fatal(err.Error())
	}
	return
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
