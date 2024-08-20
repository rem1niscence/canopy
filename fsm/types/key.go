package types

import (
	"encoding/binary"
	"fmt"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"math"
)

const (
	DAO_Pool_ID uint64 = math.MaxUint16
	FEE_Pool_ID uint64 = math.MaxUint16 - 1
)

var ReservedIDs = []uint64{
	lib.CANOPY_COMMITTEE_ID,
	DAO_Pool_ID,
	FEE_Pool_ID,
}

var (
	accountPrefix       = []byte{1}
	poolPrefix          = []byte{2}
	validatorPrefix     = []byte{3}
	committeePrefix     = []byte{4}
	unstakePrefix       = []byte{5}
	pausedPrefix        = []byte{6}
	paramsPrefix        = []byte{7}
	nonSignerPrefix     = []byte{8}
	doubleSignersPrefix = []byte{9}
	producersPrefix     = []byte{10}
	supplyPrefix        = []byte{11}
	delegatePrefix      = []byte{12}
	proposalsPrefix     = []byte{13}
)

func KeyForAccount(address crypto.AddressI) []byte { return append(accountPrefix, address.Bytes()...) }
func KeyForPool(n uint64) []byte                   { return append(poolPrefix, formatUint64(n)...) }
func KeyForValidator(addr crypto.AddressI) []byte  { return append(validatorPrefix, addr.Bytes()...) }
func KeyForParams(s string) []byte                 { return append(paramsPrefix, []byte(prefixForParamSpace(s))...) }
func KeyForNonSigner(a []byte) []byte              { return append(nonSignerPrefix, a...) }
func KeyForDoubleSigner(h, cID uint64, a []byte) []byte {
	return append(DoubleSignersPrefix(h, cID), a...)
}
func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(UnstakingPrefix(height), address.Bytes()...)
}
func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(PausedPrefix(maxPausedHeight), address.Bytes()...)
}
func KeyForCommittee(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, stake)
	return append(append(CommitteePrefix(committeeID), b...), addr.Bytes()...)
}
func KeyForDelegate(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, stake)
	return append(append(DelegatePrefix(committeeID), b...), addr.Bytes()...)
}
func AccountPrefix() []byte   { return accountPrefix }
func PoolPrefix() []byte      { return poolPrefix }
func SupplyPrefix() []byte    { return supplyPrefix }
func ValidatorPrefix() []byte { return validatorPrefix }
func NonSignerPrefix() []byte { return nonSignerPrefix }
func DoubleSignersPrefix(h, cID uint64) []byte {
	return append(append(doubleSignersPrefix, formatUint64(cID)...), formatUint64(h)...)
}
func DoubleSignerEnabledByte() []byte      { return doubleSignersPrefix }
func UnstakingPrefix(height uint64) []byte { return append(unstakePrefix, formatUint64(height)...) }
func PausedPrefix(height uint64) []byte    { return append(pausedPrefix, formatUint64(height)...) }
func ProducersPrefix() []byte              { return producersPrefix }
func CommitteePrefix(id uint64) []byte     { return append(committeePrefix, formatUint64(id)...) }
func DelegatePrefix(id uint64) []byte      { return append(delegatePrefix, formatUint64(id)...) }
func ProposalsPrefix() []byte              { return proposalsPrefix }
func AddressFromKey(k []byte) (crypto.AddressI, lib.ErrorI) {
	n := len(k)
	if n <= crypto.AddressSize {
		return nil, ErrInvalidAddressKey(k)
	}
	return crypto.NewAddressFromBytes(k[n-crypto.AddressSize:]), nil
}

func formatUint64(u uint64) []byte {
	return []byte(fmt.Sprintf("/%d/", u))
}
