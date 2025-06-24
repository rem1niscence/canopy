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

func TestGenerate20KAccounts(t *testing.T) {
	feeAmount := uint64(10000)
	numAccounts, txPerAccount := 20_000, 100
	amountInSend := 1000 + feeAmount
	fmt.Println("Starting")
	privateKeys := make([]string, 0)
	txs := make([]string, 0)
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
			jsonBz, er := lib.MarshalJSON(tx)
			require.NoError(t, er)
			txs = append(txs, string(jsonBz))
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
	require.NoError(t, os.MkdirAll("./json/", 0777))
	validatorBz, _ := json.Marshal(blsKey.String())
	require.NoError(t, os.WriteFile("json/validator_key.json", validatorBz, 0777))
	genesisBz, _ := json.MarshalIndent(j, "", "  ")
	require.NoError(t, os.WriteFile("json/genesis.json", genesisBz, 0777))
	txsBz, _ := json.Marshal(txs)
	require.NoError(t, os.WriteFile("json/txs.json", txsBz, 0777))
	accountKeysBz, _ := json.MarshalIndent(privateKeys, "", "  ")
	require.NoError(t, os.WriteFile("json/account_keys.json", accountKeysBz, 0777))
}
