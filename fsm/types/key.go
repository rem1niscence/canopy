package types

import (
	"bytes"
	"encoding/binary"
	"github.com/ginchuco/canopy/lib"
	"github.com/ginchuco/canopy/lib/crypto"
	"math"
)

var ReservedIDs = []uint64{
	lib.CanopyCommitteeId,
	lib.DAOPoolID,
}

const EscrowPoolAddend = math.MaxUint16

var (
	accountPrefix        = []byte{1}  // store key prefix for accounts
	poolPrefix           = []byte{2}  // store key prefix for pools
	validatorPrefix      = []byte{3}  // store key prefix for validators
	committeePrefix      = []byte{4}  // store key prefix for validators in committees
	unstakePrefix        = []byte{5}  // store key prefix for validators currently unstaking
	pausedPrefix         = []byte{6}  // store key prefix for validators currently paused
	paramsPrefix         = []byte{7}  // store key prefix for governance parameters
	nonSignerPrefix      = []byte{8}  // store key prefix for validators who have missed signing QCs
	lastProposersPrefix  = []byte{9}  // store key prefix for the last proposers
	supplyPrefix         = []byte{10} // store key prefix for the supply count
	delegatePrefix       = []byte{11} // store key prefix for the validators who are delegating for committees
	committeesDataPrefix = []byte{12} // store key prefix for Quorum Certificate proposals before they are paid
	orderBookPrefix      = []byte{13} // store key prefix for 'sell orders' before they are bid on
)

func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(UnstakingPrefix(height), lib.Delimit(address.Bytes())...)
}
func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(PausedPrefix(maxPausedHeight), lib.Delimit(address.Bytes())...)
}
func KeyForCommittee(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	return append(CommitteePrefix(committeeID), lib.Delimit(formatUint64(stake), addr.Bytes())...)
}
func KeyForDelegate(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	return append(DelegatePrefix(committeeID), lib.Delimit(formatUint64(stake), addr.Bytes())...)
}
func KeyForAccount(addr crypto.AddressI) []byte   { return lib.Delimit(accountPrefix, addr.Bytes()) }
func KeyForPool(n uint64) []byte                  { return lib.Delimit(poolPrefix, formatUint64(n)) }
func KeyForValidator(addr crypto.AddressI) []byte { return lib.Delimit(validatorPrefix, addr.Bytes()) }
func KeyForParams(s string) []byte                { return lib.Delimit(paramsPrefix, []byte(prefixForParamSpace(s))) }
func KeyForNonSigner(a []byte) []byte             { return lib.Delimit(nonSignerPrefix, a) }
func KeyForOrderBook(id uint64) []byte            { return lib.Delimit(orderBookPrefix, formatUint64(id)) }
func AccountPrefix() []byte                       { return lib.Delimit(accountPrefix) }
func PoolPrefix() []byte                          { return lib.Delimit(poolPrefix) }
func SupplyPrefix() []byte                        { return lib.Delimit(supplyPrefix) }
func ValidatorPrefix() []byte                     { return lib.Delimit(validatorPrefix) }
func NonSignerPrefix() []byte                     { return lib.Delimit(nonSignerPrefix) }
func UnstakingPrefix(height uint64) []byte        { return lib.Delimit(unstakePrefix, formatUint64(height)) }
func PausedPrefix(height uint64) []byte           { return lib.Delimit(pausedPrefix, formatUint64(height)) }
func LastProposersPrefix() []byte                 { return lib.Delimit(lastProposersPrefix) }
func CommitteePrefix(id uint64) []byte            { return lib.Delimit(committeePrefix, formatUint64(id)) }
func DelegatePrefix(id uint64) []byte             { return lib.Delimit(delegatePrefix, formatUint64(id)) }
func OrderBookPrefix() []byte                     { return lib.Delimit(orderBookPrefix) }
func CommitteesDataPrefix() []byte                { return lib.Delimit(committeesDataPrefix) }
func AddressFromKey(k []byte) (crypto.AddressI, lib.ErrorI) {
	n := len(k)
	delimLen := len(lib.Delimiter)
	if n <= crypto.AddressSize+delimLen {
		return nil, ErrInvalidKey(k)
	}
	return crypto.NewAddressFromBytes(k[n-crypto.AddressSize-delimLen : n-delimLen]), nil
}

func IdFromKey(k []byte) (uint64, lib.ErrorI) {
	if bytes.Count(k, []byte("/")) != 2 {
		return 0, ErrInvalidKey(k)
	}
	return binary.BigEndian.Uint64(bytes.Split(k, []byte("/"))[1]), nil
}

func formatUint64(u uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, u)
	return b
}
