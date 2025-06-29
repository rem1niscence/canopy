package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/units"
	"github.com/canopy-network/canopy/fsm"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestGenerateTxs(t *testing.T) {
	feeAmount, blockIndex := uint64(10000), 0
	numAccounts, txPerAccount := 20_000, 1000
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
			toAddress := make([]byte, 20)
			_, er := rand.Read(toAddress)
			require.NoError(t, er)
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
	fmt.Println("Validator Key:", blsKey.String())
	params := fsm.DefaultParams()
	params.Consensus.BlockSize = uint64(128 * units.MB)
	j := &fsm.GenesisState{
		Accounts: accounts,
		Validators: []*fsm.Validator{{
			Address:      blsKey.PublicKey().Address().Bytes(),
			PublicKey:    blsKey.PublicKey().Bytes(),
			Committees:   []uint64{lib.CanopyChainId},
			NetAddress:   "tcp://localhost",
			StakedAmount: 1,
			Output:       blsKey.PublicKey().Address().Bytes(),
			Compound:     true,
		}},
		Params: params,
	}
	fmt.Println("Writing files")
	require.NoError(t, os.MkdirAll("./data/", 0777))
	validatorBz, _ := json.Marshal(blsKey.String())
	require.NoError(t, os.WriteFile("./data/validator_key.json", validatorBz, 0777))
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
