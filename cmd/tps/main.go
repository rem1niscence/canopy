package main

import (
	"encoding/json"
	"fmt"
	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
	"os"
	"sync"
	"time"
)

const (
	txsPerBlock = 200_000
	totalTxs    = 20_000_000
	numWorkers  = 64 // You can tune this based on CPU & RPC server load
	rpcURL      = "http://localhost:50002"
	adminRPCURL = "http://localhost:50003"
)

// TODO mac users:
// - Allow more ports: sudo sysctl -w net.inet.ip.portrange.first=32768
// - Allow more FDs: ulimit -n 65535

func main() {
	fmt.Println("Creating new RPC client")
	client := rpc.NewClient(rpcURL, adminRPCURL)

	fmt.Println("Loading transactions from JSON file")
	txsFile, err := os.Open("cmd/tps/data/txs.proto")
	if err != nil {
		panic(err)
	}
	defer txsFile.Close()

	var txs []string
	if err := json.NewDecoder(txsFile).Decode(&txs); err != nil {
		panic(err)
	}
	fmt.Println("Done loading", len(txs), "transactions")

	var (
		lastHeight uint64
		index      uint64
		mu         sync.Mutex // to protect `index` in concurrent access
	)

	for range time.Tick(100 * time.Millisecond) {
		height, err := client.Height()
		if err != nil || *height <= lastHeight || *height <= 1 {
			continue
		}
		lastHeight = *height
		time.Sleep(2 * time.Second)
		fmt.Println("Submitting", txsPerBlock, "txs for height", lastHeight)

		var wg sync.WaitGroup
		tasks := make(chan string, txsPerBlock)

		// launch workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for txStr := range tasks {
					var raw json.RawMessage
					if err := lib.UnmarshalJSON([]byte(txStr), &raw); err != nil {
						fmt.Println("JSON ERROR:", err)
						continue
					}
					if _, err := client.TransactionJSON(raw); err != nil {
						fmt.Println("RPC ERROR:", err)
					}
				}
			}()
		}

		// Enqueue work
		for i := 0; i < txsPerBlock; i++ {
			mu.Lock()
			if index >= totalTxs {
				mu.Unlock()
				close(tasks)
				wg.Wait()
				fmt.Println("Exhausted all transactions")
				os.Exit(0)
			}
			tx := txs[index]
			index++
			mu.Unlock()
			tasks <- tx
		}

		close(tasks)
		wg.Wait()
		fmt.Println("Done submitting block", lastHeight)
	}
}
