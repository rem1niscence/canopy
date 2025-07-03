package main

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	math "math/rand"
	"os"
	"testing"
	"time"
)

func TestGenerateTxs(t *testing.T) {
	t.SkipNow()
	feeAmount, blockIndex := uint64(10000), 0
	numAccounts, txPerAccount := 1000, 20_000
	amountInSend := 1000 + feeAmount
	fmt.Println("Starting")
	privateKeys := make([]string, numAccounts)
	blk := new(lib.Block)
	blk.Transactions = make([][]byte, 0)
	accounts := make([]*fsm.Account, 0)
	for i := 0; i < numAccounts; i++ {
		fmt.Println("Generating txs for account", i)
		privateKey, e := crypto.NewEd25519PrivateKey()
		require.NoError(t, e)
		privateKeys[i] = privateKey.String()
		accounts = append(accounts, &fsm.Account{
			Address: privateKey.PublicKey().Address().Bytes(),
			Amount:  amountInSend * uint64(txPerAccount),
		})
		for j := 0; j < txPerAccount; j++ {
			toAddress := accounts[math.Intn(len(accounts))].Address
			//toAddress := make([]byte, 20)
			//_, er := rand.Read(toAddress)
			//require.NoError(t, er)
			tx, er := fsm.NewSendTransaction(privateKey, crypto.NewAddress(toAddress), 1000, 1, 1, feeAmount, 1, "")
			require.NoError(t, er)
			protoBz, er := lib.Marshal(tx)
			require.NoError(t, er)
			blk.Transactions = append(blk.Transactions, protoBz)
			// append to a file
			if len(blk.Transactions) == txsPerBlock {
				// write this block to file
				bz, err := lib.Marshal(blk)
				require.NoError(t, err)
				fileName := fmt.Sprintf("./data/txs_block_%05d.proto", blockIndex)
				require.NoError(t, os.WriteFile(fileName, bz, 0777))
				// reset
				blockIndex++
				blk = &lib.Block{}
				blk.Transactions = make([][]byte, 0, txsPerBlock)
			}
		}
	}
	fmt.Println("Generating genesis file")
	blsKey, err := crypto.NewBLS12381PrivateKey()
	require.NoError(t, err)
	blsKey2, err := crypto.NewBLS12381PrivateKey()
	require.NoError(t, err)
	params := fsm.DefaultParams()
	params.Consensus.BlockSize = uint64(128 * units.MB)
	j := &fsm.GenesisState{
		Accounts: accounts,
		Validators: []*fsm.Validator{{
			Address:      blsKey.PublicKey().Address().Bytes(),
			PublicKey:    blsKey.PublicKey().Bytes(),
			Committees:   []uint64{lib.CanopyChainId},
			NetAddress:   "tcp://node-1",
			StakedAmount: 1,
			Output:       blsKey.PublicKey().Address().Bytes(),
			Compound:     true,
		},
			{
				Address:      blsKey2.PublicKey().Address().Bytes(),
				PublicKey:    blsKey2.PublicKey().Bytes(),
				Committees:   []uint64{lib.CanopyChainId},
				NetAddress:   "tcp://node-2",
				StakedAmount: 1,
				Output:       blsKey2.PublicKey().Address().Bytes(),
				Compound:     true,
			}},
		Params: params,
	}
	fmt.Println("Writing files")
	require.NoError(t, os.MkdirAll("./data/", 0777))
	validatorBz, _ := json.Marshal(blsKey.String())
	require.NoError(t, os.WriteFile("./data/n1_validator_key.json", validatorBz, 0777))
	validator2Bz, _ := json.Marshal(blsKey2.String())
	require.NoError(t, os.WriteFile("./data/n2_validator_key.json", validator2Bz, 0777))
	genesisBz, _ := json.MarshalIndent(j, "", "  ")
	require.NoError(t, os.WriteFile("./data/genesis.json", genesisBz, 0777))
	txsBz, _ := lib.Marshal(blk)
	// write any remaining transactions
	if len(blk.Transactions) > 0 {
		fileName := fmt.Sprintf("./data/txs_block_%05d.proto", blockIndex)
		require.NoError(t, os.WriteFile(fileName, txsBz, 0777))
	}
	accountKeysBz, _ := json.MarshalIndent(privateKeys, "", "  ")
	require.NoError(t, os.WriteFile("./data/account_keys.json", accountKeysBz, 0777))
}

func TestT(t *testing.T) {
	blk := new(lib.Block)

	for range time.Tick(1 * time.Second) {
		blockIndex := 0

		// load corresponding block file
		//home, _ := os.UserHomeDir()
		fileName := fmt.Sprintf(fmt.Sprintf("./data/txs_block_%05d.proto", blockIndex))
		fmt.Printf("Loading %s\n", fileName)
		txsFile, err := os.ReadFile(fileName)
		if err != nil {
			fmt.Printf("Error reading file: %s\n", err)
			return
		}
		if err = lib.Unmarshal(txsFile, blk); err != nil {
			fmt.Printf("Error unmarshaling: %s\n", err)
			return
		}
		maxTransactions := min(len(blk.Transactions), 200_000)
		fmt.Printf("Loaded %d txs from block %d using %d\n", len(blk.Transactions), blockIndex, maxTransactions)
		var size int
		for i, tx := range blk.Transactions {
			if i >= maxTransactions {
				break
			}
			size += len(tx)
		}
		fmt.Println(size)
		fmt.Println("Submitted block", blockIndex)
	}
}
