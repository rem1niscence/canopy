package fsm

import (
	"github.com/ginchuco/ginchu/fsm/types"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"github.com/ginchuco/ginchu/store"
	"github.com/stretchr/testify/require"
	"testing"
)

func newSingleAccountStateMachine(t *testing.T) StateMachine {
	sm := newTestStateMachine(t)
	keyGroup := newTestKeyGroup(t)
	require.NoError(t, sm.SetParams(types.DefaultParams()))
	require.NoError(t, sm.SetAccount(&types.Account{
		Address: keyGroup.Address.Bytes(),
		Amount:  1000000,
	}))
	require.NoError(t, sm.HandleMessageStake(&types.MessageStake{
		PublicKey:     keyGroup.PublicKey.Bytes(),
		Amount:        1000000,
		Committees:    []uint64{lib.CanopyCommitteeId},
		NetAddress:    "http://localhost:80",
		OutputAddress: keyGroup.Address.Bytes(),
		Delegate:      false,
		Compound:      true,
	}))
	require.NoError(t, sm.SetParams(types.DefaultParams()))
	return sm
}

func newTestStateMachine(t *testing.T) StateMachine {
	log := lib.NewDefaultLogger()
	db, err := store.NewStoreInMemory(log)
	db.Commit()
	require.NoError(t, err)
	sm := StateMachine{
		store:             db,
		ProtocolVersion:   0,
		NetworkID:         0,
		height:            2,
		vdfIterations:     0,
		slashTracker:      types.NewSlashTracker(),
		proposeVoteConfig: types.AcceptAllProposals,
		Config:            lib.Config{},
		log:               log,
	}
	require.NoError(t, sm.SetParams(types.DefaultParams()))
	return sm
}

func newTestAddress(t *testing.T, variation ...int) crypto.AddressI {
	kg := newTestKeyGroup(t, variation...)
	return kg.Address
}

func newTestAddressBytes(t *testing.T, variation ...int) []byte {
	return newTestAddress(t, variation...).Bytes()
}

func newTestPublicKey(t *testing.T, variation ...int) crypto.PublicKeyI {
	kg := newTestKeyGroup(t, variation...)
	return kg.PublicKey
}

func newTestPublicKeyBytes(t *testing.T, variation ...int) []byte {
	return newTestPublicKey(t, variation...).Bytes()
}

func newTestKeyGroup(t *testing.T, variation ...int) *crypto.KeyGroup {
	var (
		key  crypto.PrivateKeyI
		err  error
		keys = []string{
			"01553a101301cd7019b78ffa1186842dd93923e563b8ae22e2ab33ae889b23ee",
			"1b6b244fbdf614acb5f0d00a2b56ffcbe2aa23dabd66365dffcd3f06491ae50a",
			"2ee868f74134032eacba191ca529115c64aa849ac121b75ca79b37420a623036",
			"3e3ab94c10159d63a12cb26aca4b0e76070a987d49dd10fc5f526031e05801da",
			"479839d3edbd0eefa60111db569ded6a1a642cc84781600f0594bd8d4a429319",
			"51eb5eb6eca0b47c8383652a6043aadc66ddbcbe240474d152f4d9a7439eae42",
			"637cb8e916bba4c1773ed34d89ebc4cb86e85c145aea5653a58de930590a2aa4",
			"7235e5757e6f52e6ae4f9e20726d9c514281e58e839e33a7f667167c524ff658"}
	)

	if len(variation) == 1 {
		key, err = crypto.NewBLSPrivateKeyFromString(keys[variation[0]])
	} else {
		key, err = crypto.NewBLSPrivateKeyFromString(keys[0])
	}
	require.NoError(t, err)
	return crypto.NewKeyGroup(key)
}

func newTestKeyGroups(t *testing.T, count int) (groups []*crypto.KeyGroup) {
	for i := 0; i < count; i++ {
		groups = append(groups, newTestKeyGroup(t, i))
	}
	return
}

// testQCParams are the associate parameters needed to generate a testQC
type testQCParams struct {
	height        uint64
	idxSigned     map[int]bool
	committeeKeys []*crypto.KeyGroup
	committee     []*types.Validator
	results       *lib.CertificateResult
}

// newTestQC is a utility function for this test to generate various quorum certificates in the test cases
func newTestQC(t *testing.T, params testQCParams) (qc *lib.QuorumCertificate) {
	// convert committee members to consensus validators
	var vals []*lib.ConsensusValidator
	for _, m := range params.committee {
		vals = append(vals, &lib.ConsensusValidator{PublicKey: m.PublicKey, VotingPower: m.StakedAmount})
	}
	// create a validator set object in order to generate a multi-public key for the set
	validatorSet, err := lib.NewValidatorSet(&lib.ConsensusValidators{ValidatorSet: vals})
	require.NoError(t, err)
	// create the 'justification' object
	justification := validatorSet.MultiKey.Copy()
	// create the certificate results object to put in the QC
	// create the QC object
	qc = &lib.QuorumCertificate{
		Header: &lib.View{
			Height:          params.height,
			CommitteeHeight: params.height,
			CommitteeId:     lib.CanopyCommitteeId,
		},
		Results:     params.results,
		ResultsHash: params.results.Hash(),
		BlockHash:   crypto.Hash([]byte("some block that's not included here")),
	}
	// generate the bytes to be signed in the justification (multi-key)
	bytesToBeSigned := qc.SignBytes()
	// have the 'signers' sign the justification (multi-key)
	for i, s := range params.committeeKeys {
		if params.idxSigned[i] {
			require.NoError(t, justification.AddSigner(s.PrivateKey.Sign(bytesToBeSigned), i))
		}
	}
	// aggregate the signature
	aggregateSignatures, e := justification.AggregateSignatures()
	require.NoError(t, e)
	// wrap in object
	qc.Signature = &lib.AggregateSignature{
		Signature: aggregateSignatures,
		Bitmap:    justification.Bitmap(),
	}
	return
}
