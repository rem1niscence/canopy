package main

import (
	"encoding/json"
	"fmt"
	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
	"os"
	"time"
)

const txsPerBlock, totalTxs = 200_000, 2_000_000
const rpcURL, adminRPCURL = "http://localhost:50002", "http://localhost:50003"

func main() {
	fmt.Println("Creating new rpc client")
	client := rpc.NewClient(rpcURL, adminRPCURL)
	if _, err := client.Height(); err != nil {
		panic(err)
	}
	fmt.Println("Loading transactions from json file")
	txsFile, err := os.Open("cmd/tps/json/txs.json")
	if err != nil {
		panic(err)
	}
	defer txsFile.Close()
	var txs []string
	decoder := json.NewDecoder(txsFile)
	if err = decoder.Decode(&txs); err != nil {
		panic(err)
	}
	fmt.Println("Done loading transactions from json file")
	var lastHeight, index uint64
	for range time.Tick(100 * time.Millisecond) {
		height, e := client.Height()
		if e != nil {
			continue
		}
		if *height > lastHeight && *height > 1 {
			lastHeight = *height
		} else {
			continue
		}
		fmt.Println("Submitting", txsPerBlock, "txs for height", lastHeight)
		for i := 0; i < txsPerBlock; func() { i++; index++ }() {
			if index >= totalTxs {
				fmt.Println("Exhausted all transactions")
				os.Exit(0)
			}
			var raw json.RawMessage
			if err = lib.UnmarshalJSON([]byte(txs[index]), &raw); err != nil {
				panic(err)
			}
			if _, err = client.TransactionJSON(raw); err != nil {
				fmt.Println("RPC ERROR: ", err.Error())
			}
		}
		fmt.Println("Done submitting this block")
	}
}
