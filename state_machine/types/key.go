package types

import "github.com/ginchuco/ginchu/crypto"

var (
	accountPrefix = []byte{0x01}
	poolPrefix    = []byte{0x02}
	valPrefix     = []byte{0x03}
)

func KeyForAccount(address crypto.AddressI) []byte   { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(name string) []byte                  { return append(poolPrefix, []byte(name)...) }
func KeyForValidator(address crypto.AddressI) []byte { return append(valPrefix, address.Bytes()...) }

func AccountPrefix() []byte   { return accountPrefix }
func ValidatorPrefix() []byte { return valPrefix }
