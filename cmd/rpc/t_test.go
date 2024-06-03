package rpc

import (
	"fmt"
	_ "github.com/Cside/jsondiff"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	_ "github.com/josephburnett/jd/lib"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestRPCClient(t *testing.T) {
	config := lib.DefaultConfig()
	client := NewClient("http://"+localhost, config.RPCPort, config.AdminPort)
	keystore, err := client.Keystore()
	require.NoError(t, err)
	for k, _ := range *keystore {
		fmt.Println(k)
	}
}

func TestRPCClientK2(t *testing.T) {
	config := lib.DefaultConfig()
	client := NewClient("http://"+localhost, config.RPCPort, config.AdminPort)
	fmt.Println(client.KeystoreGet("07907232cb8198d07cda010a003ca00a6ea532d4", ""))
}

func TestConfig(t *testing.T) {
	k, err := crypto.NewBLSPrivateKeyFromFile(filepath.Join(lib.DefaultDataDirPath(), lib.ValKeyPath))
	require.NoError(t, err)
	fmt.Println(k.PublicKey().String())
	fmt.Println(k.PublicKey().Address().String())
}

func TestB(t *testing.T) {
	config := lib.DefaultConfig()
	client := NewClient("http://"+localhost, config.RPCPort, config.AdminPort)
	res, err := client.TransactionsBySender("1fe1e32edc41d688d83c14d94a8dd870a29f4da9", lib.PageParams{
		PageNumber: 0,
		PerPage:    5,
	})
	require.NoError(t, err)
	c := res.Results.(*lib.TxResults)
	a := ([]*lib.TxResult)(*c)
	fmt.Println(a[1].Index)
}

func TestProposal(t *testing.T) {
	config := lib.DefaultConfig()
	client := NewClient("http://"+localhost, config.RPCPort, config.AdminPort)
	json := `{
  "type": "send",
  "msg": {
    "from_address": "1fe1e32edc41d688d83c14d94a8dd870a29f4da9",
    "to_address": "1fe1e32edc41d688d83c14d94a8dd870a29f4da9",
    "amount": 1
  },
  "signature": {
    "public_key": "a0807d42a5adfa6ef8ac3cac37a2651e838407b20986db170c5caa88b9c0c7b77e7b3ededd75242261fa6cbc3d7b0165",
    "signature": "a8f07f70950bc9313e40878b508d4cd0145af8eafb28254ae83c43e63967e5a17bde450a3e7a8cc1d352985854f522811516e47ba057f14e3aaee880f41a41f50ae7368c458067e32e7d37d6311188db2efecc1afa8a5ee7a75db1c147c289e0"
  },
  "sequence": 15,
  "fee": 10000
}`
	hash, err := client.TransactionJSON([]byte(json))
	require.NoError(t, err)
	fmt.Println(hash)
}

func TestPeerInfo(t *testing.T) {
	addr, _ := lib.NewHexBytesFromString("1fe1e32edc41d688d83c14d94a8dd870a29f4da9")
	test := txRequest{
		Amount:     1,
		NetAddress: "",
		Output:     "1fe1e32edc41d688d83c14d94a8dd870a29f4da9",
		Sequence:   0,
		Fee:        0,
		Submit:     false,
		addressRequest: addressRequest{
			Address: addr,
		},
		passwordRequest: passwordRequest{
			Password: "test",
		},
		txChangeParamRequest: txChangeParamRequest{},
	}
	fmt.Println(lib.MarshalJSONIndentString(test))
}
