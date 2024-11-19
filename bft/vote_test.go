package bft

import (
	"bytes"
	"github.com/canopy-network/canopy/lib"
	"github.com/canopy-network/canopy/lib/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddVote(t *testing.T) {
	// pre-define some validators to test with
	_, keys, _ := newTestValSet(t, 3)
	// define test cases
	tests := []struct {
		name    string
		detail  string
		preAdd  []*Message
		message *Message
		error   string
	}{
		{
			name:   "duplicate voter",
			detail: "a message for this view was already received from this peer",
			preAdd: []*Message{
				{
					Qc: &lib.QuorumCertificate{
						Header: &lib.View{
							Phase: lib.Phase_ELECTION_VOTE,
						},
					},
					Signature: &lib.Signature{
						PublicKey: keys[0].PublicKey().Bytes(),
						Signature: bytes.Repeat([]byte("F"), 96),
					},
				},
			},
			message: &Message{
				Qc: &lib.QuorumCertificate{
					Header: &lib.View{
						Phase: lib.Phase_ELECTION_VOTE,
					},
				},
				Signature: &lib.Signature{
					PublicKey: keys[0].PublicKey().Bytes(),
					Signature: bytes.Repeat([]byte("F"), 96),
				},
			},
			error: "duplicate vote",
		},
		{
			name:   "vote added",
			detail: "this vote message is valid and unique, so no error",
			preAdd: []*Message{},
			message: &Message{
				Qc: &lib.QuorumCertificate{
					Header: &lib.View{
						Phase: lib.Phase_ELECTION_VOTE,
					},
				},
				Signature: &lib.Signature{
					PublicKey: keys[0].PublicKey().Bytes(),
					Signature: bytes.Repeat([]byte("F"), 96),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// initialize a bft object to test with
			consensus := newTestConsensus(t, ElectionVote, 3)
			// pre-add the messages
			for _, add := range test.preAdd {
				require.NoError(t, consensus.bft.AddVote(add))
			}
			// execute the function call
			err := consensus.bft.AddVote(test.message)
			// validate if an error is expected
			require.Equal(t, err != nil, test.error != "", err)
			// validate actual error if any
			if err != nil {
				require.ErrorContains(t, err, test.error, err)
				return
			}
			// make a convenience variable for the view of the message
			v := test.message.Qc.Header
			// ensure the message was added
			messages := consensus.bft.Votes[v.Round][phaseToString(v.Phase)]
			// calculate the payload
			payload := crypto.HashString(test.message.SignBytes())
			require.EqualExportedValues(t, test.message, messages[payload].Vote)
		})
	}
}
