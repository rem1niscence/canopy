package lib

import (
	"bytes"
	"encoding/json"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCertificateCheckBasic(t *testing.T) {
	// predefine qc results
	results := &CertificateResult{
		RewardRecipients: &RewardRecipients{
			PaymentPercents: []*PaymentPercents{
				{
					Address: newTestAddressBytes(t),
					Percent: 100,
				},
			},
		},
	}
	// define test cases
	tests := []struct {
		name   string
		detail string
		qc     *QuorumCertificate
		error  string
	}{
		{
			name:   "empty",
			detail: "the qc is nil or empty",
			qc:     nil,
			error:  "empty quorum certificate",
		},
		{
			name:   "view empty",
			detail: "the qc view is nil or empty",
			qc: &QuorumCertificate{
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "empty view",
		},
		{
			name:   "invalid block hash",
			detail: "the block hash is an invalid length",
			qc: &QuorumCertificate{
				Header:      &View{},
				ResultsHash: crypto.Hash([]byte("hash")),
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "invalid block hash",
		},
		{
			name:   "invalid results hash",
			detail: "the results hash is an invalid length",
			qc: &QuorumCertificate{
				Header:      &View{},
				BlockHash:   crypto.Hash([]byte("h")),
				ResultsHash: []byte("wrong_length"),
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "invalid results hash",
		},
		{
			name:   "mismatch results hash",
			detail: "the results hash does not match the results",
			qc: &QuorumCertificate{
				Header:      &View{},
				BlockHash:   crypto.Hash([]byte("h")),
				ResultsHash: crypto.Hash([]byte("h")),
				Results:     results,
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "mismatch results hash",
		},
		{
			name:   "mismatch block hash",
			detail: "the block hash does not match the block",
			qc: &QuorumCertificate{
				Header:      &View{},
				Block:       []byte("b"),
				BlockHash:   crypto.Hash([]byte("h")),
				ResultsHash: results.Hash(),
				Results:     results,
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "mismatch block hash",
		},
		{
			name:   "empty aggregate signature",
			detail: "the aggregate signature is empty",
			qc: &QuorumCertificate{
				Header:      &View{},
				Block:       []byte("b"),
				BlockHash:   crypto.Hash([]byte("b")),
				ResultsHash: results.Hash(),
				Results:     results,
				ProposerKey: newTestPublicKeyBytes(t),
			},
			error: "empty aggregate signature",
		},
		{
			name:   "no error",
			detail: "the happy path",
			qc: &QuorumCertificate{
				Header:      &View{},
				Block:       []byte("b"),
				BlockHash:   crypto.Hash([]byte("b")),
				ResultsHash: results.Hash(),
				Results:     results,
				ProposerKey: newTestPublicKeyBytes(t),
				Signature: &AggregateSignature{
					Signature: bytes.Repeat([]byte("F"), 96),
					Bitmap:    []byte("bm"),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// execute the function call
			err := test.qc.CheckBasic()
			// validate if an error is expected
			require.Equal(t, err != nil, test.error != "", err)
			// validate actual error if any
			if err != nil {
				require.ErrorContains(t, err, test.error)
			}
		})
	}
}

func TestCertificateMarshalJSON(t *testing.T) {
	// predefine qc results
	results := &CertificateResult{
		RewardRecipients: &RewardRecipients{
			PaymentPercents: []*PaymentPercents{
				{
					Address: newTestAddressBytes(t),
					Percent: 100,
				},
			},
		},
	}
	// predefine a quorum certificate
	qc := &QuorumCertificate{
		Header: &View{
			NetworkId:    1,
			CommitteeId:  CanopyCommitteeId,
			Height:       1,
			CanopyHeight: 1,
			Round:        1,
		},
		Results:     results,
		ResultsHash: results.Hash(),
		Block:       []byte("block"),
		BlockHash:   crypto.Hash([]byte("block")),
		ProposerKey: newTestPublicKeyBytes(t),
		Signature: &AggregateSignature{
			Signature: bytes.Repeat([]byte("F"), 96),
			Bitmap:    []byte("bit_map"),
		},
	}
	// convert to json bytes
	qcBytes, err := json.Marshal(qc)
	require.NoError(t, err)
	// define a new variable to unmarshal into
	got := new(QuorumCertificate)
	// convert bytes to object
	require.NoError(t, json.Unmarshal(qcBytes, got))
	// compare got vs expected
	require.EqualExportedValues(t, qc, got)
}

func TestCertificateSignBytes(t *testing.T) {
	// predefine qc results
	results := &CertificateResult{
		RewardRecipients: &RewardRecipients{
			PaymentPercents: []*PaymentPercents{
				{
					Address: newTestAddressBytes(t),
					Percent: 100,
				},
			},
		},
	}
	// predefine a quorum certificate
	qc := &QuorumCertificate{
		Header: &View{
			NetworkId:    1,
			CommitteeId:  CanopyCommitteeId,
			Height:       1,
			CanopyHeight: 1,
			Round:        1,
		},
		Results:     results,
		ResultsHash: results.Hash(),
		Block:       []byte("block"),
		BlockHash:   crypto.Hash([]byte("block")),
		ProposerKey: newTestPublicKeyBytes(t),
		Signature: &AggregateSignature{
			Signature: bytes.Repeat([]byte("F"), 96),
			Bitmap:    []byte("bit_map"),
		},
	}
	// temp variables to save values
	results, block, aggregateSignature := qc.Results, qc.Block, qc.Signature
	// remove the values from the struct
	qc.Results, qc.Block, qc.Signature = nil, nil, nil
	// calculate expected
	expected, _ := Marshal(qc)
	// add back the removed values
	qc.Results, qc.Block, qc.Signature = results, block, aggregateSignature
	// execute the function call
	got := qc.SignBytes()
	// check got vs expected
	require.Equal(t, expected, got)
}
