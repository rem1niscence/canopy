package types

import (
	"encoding/binary"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"math"
)

var ReservedIDs = []uint64{
	lib.UnknownCommitteeId,
	lib.DAOPoolID,
}

const EscrowPoolAddend = math.MaxUint16

var (
	accountPrefix          = []byte{1}  // store key prefix for accounts
	poolPrefix             = []byte{2}  // store key prefix for pools
	validatorPrefix        = []byte{3}  // store key prefix for validators
	committeePrefix        = []byte{4}  // store key prefix for validators in committees
	unstakePrefix          = []byte{5}  // store key prefix for validators currently unstaking
	pausedPrefix           = []byte{6}  // store key prefix for validators currently paused
	paramsPrefix           = []byte{7}  // store key prefix for governance parameters
	nonSignerPrefix        = []byte{8}  // store key prefix for validators who have missed signing QCs
	lastProposersPrefix    = []byte{9}  // store key prefix for the last proposers
	supplyPrefix           = []byte{10} // store key prefix for the supply count
	delegatePrefix         = []byte{11} // store key prefix for the validators who are delegating for committees
	committeesDataPrefix   = []byte{12} // store key prefix for Quorum Certificate proposals before they are paid
	orderBookPrefix        = []byte{13} // store key prefix for 'sell orders' before they are bid on
	retiredCommitteePrefix = []byte{14} // store key prefix for 'retired' (dead) committees
)

func KeyForUnstaking(height uint64, address crypto.AddressI) []byte {
	return append(UnstakingPrefix(height), lib.AppendAndLenPrefix(address.Bytes())...)
}
func KeyForPaused(maxPausedHeight uint64, address crypto.AddressI) []byte {
	return append(PausedPrefix(maxPausedHeight), lib.AppendAndLenPrefix(address.Bytes())...)
}
func KeyForCommittee(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	return append(CommitteePrefix(committeeID), lib.AppendAndLenPrefix(formatUint64(stake), addr.Bytes())...)
}
func KeyForDelegate(committeeID uint64, addr crypto.AddressI, stake uint64) []byte {
	return append(DelegatePrefix(committeeID), lib.AppendAndLenPrefix(formatUint64(stake), addr.Bytes())...)
}
func KeyForRetiredCommittee(committeeID uint64) []byte {
	return lib.AppendAndLenPrefix(retiredCommitteePrefix, formatUint64(committeeID))
}
func KeyForAccount(addr crypto.AddressI) []byte {
	return lib.AppendAndLenPrefix(accountPrefix, addr.Bytes())
}
func KeyForPool(n uint64) []byte { return lib.AppendAndLenPrefix(poolPrefix, formatUint64(n)) }
func KeyForValidator(addr crypto.AddressI) []byte {
	return lib.AppendAndLenPrefix(validatorPrefix, addr.Bytes())
}
func KeyForParams(s string) []byte {
	return lib.AppendAndLenPrefix(paramsPrefix, []byte(prefixForParamSpace(s)))
}
func KeyForNonSigner(a []byte) []byte { return lib.AppendAndLenPrefix(nonSignerPrefix, a) }
func KeyForOrderBook(id uint64) []byte {
	return lib.AppendAndLenPrefix(orderBookPrefix, formatUint64(id))
}
func AccountPrefix() []byte   { return lib.AppendAndLenPrefix(accountPrefix) }
func PoolPrefix() []byte      { return lib.AppendAndLenPrefix(poolPrefix) }
func SupplyPrefix() []byte    { return lib.AppendAndLenPrefix(supplyPrefix) }
func ValidatorPrefix() []byte { return lib.AppendAndLenPrefix(validatorPrefix) }
func NonSignerPrefix() []byte { return lib.AppendAndLenPrefix(nonSignerPrefix) }
func UnstakingPrefix(height uint64) []byte {
	return lib.AppendAndLenPrefix(unstakePrefix, formatUint64(height))
}
func PausedPrefix(height uint64) []byte {
	return lib.AppendAndLenPrefix(pausedPrefix, formatUint64(height))
}
func LastProposersPrefix() []byte { return lib.AppendAndLenPrefix(lastProposersPrefix) }
func CommitteePrefix(id uint64) []byte {
	return lib.AppendAndLenPrefix(committeePrefix, formatUint64(id))
}
func DelegatePrefix(id uint64) []byte {
	return lib.AppendAndLenPrefix(delegatePrefix, formatUint64(id))
}
func OrderBookPrefix() []byte         { return lib.AppendAndLenPrefix(orderBookPrefix) }
func CommitteesDataPrefix() []byte    { return lib.AppendAndLenPrefix(committeesDataPrefix) }
func RetiredCommitteesPrefix() []byte { return lib.AppendAndLenPrefix(retiredCommitteePrefix) }
func AddressFromKey(k []byte) (crypto.AddressI, lib.ErrorI) {
	segments := lib.DecodeLengthPrefixed(k)
	return crypto.NewAddressFromBytes(segments[len(segments)-1]), nil
}

func IdFromKey(k []byte) (uint64, lib.ErrorI) {
	segments := lib.DecodeLengthPrefixed(k)
	if len(segments) != 2 {
		return 0, ErrInvalidKey(k)
	}
	return binary.BigEndian.Uint64(segments[1]), nil
}

func formatUint64(u uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, u)
	return b
}
