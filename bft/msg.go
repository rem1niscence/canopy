package bft

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"google.golang.org/protobuf/proto"
)

// HandleMessage handles an incoming consensus message from a Validator peer
func (c *Consensus) HandleMessage(message proto.Message) lib.ErrorI {
	// Ensure is a valid `Consensus Message` type
	switch msg := message.(type) {
	case *Message:
		if msg.Signature == nil {
			return lib.ErrEmptySignature()
		}
		if _, err := c.ValidatorSet.GetValidator(msg.Signature.PublicKey); err != nil {
			return err
		}
		switch {
		case msg.IsReplicaMessage() || msg.IsPacemakerMessage():
			if err := c.CheckReplicaMessage(msg); err != nil {
				c.log.Errorf("Received invalid vote from %s", lib.BytesToString(msg.Signature.PublicKey))
				return err
			}
			c.log.Debugf("Received %s message from replica: %s", msg.Qc.Header.ToString(), lib.BzToTruncStr(msg.Signature.PublicKey))
			if msg.IsPacemakerMessage() {
				return c.AddPacemakerMessage(msg)
			}
			return c.AddVote(msg)
		case msg.IsProposerMessage():
			partialQC, err := c.CheckProposerMessage(msg)
			if err != nil {
				c.log.Errorf("Received invalid proposal from %s", lib.BytesToString(msg.Signature.PublicKey))
				return err
			}
			c.log.Debugf("Received %s message from proposer: %s", msg.Header.ToString(), lib.BzToTruncStr(msg.Signature.PublicKey))
			if partialQC {
				c.log.Errorf("Received partial QC from proposer %s", lib.BzToTruncStr(msg.Signature.PublicKey))
				return c.AddPartialQC(msg)
			}
			return c.AddProposal(msg)
		}
	}
	return ErrUnknownConsensusMsg(message)
}

func (c *Consensus) CheckProposerMessage(x *Message) (isPartialQC bool, err lib.ErrorI) {
	if err = x.checkBasic(); err != nil { // TODO ensure the round is correct when receiving a proposal
		return false, err
	}
	if c.ProposerKey != nil {
		if !bytes.Equal(c.ProposerKey, x.Signature.PublicKey) {
			return false, lib.ErrInvalidProposerPubKey()
		}
	}
	if x.Header.Phase == Election {
		if x.Header.Height != c.Height {
			return false, lib.ErrWrongHeight()
		}
		if err = checkSignatureBasic(x.Vrf); err != nil {
			return false, err
		}
		if !bytes.Equal(x.Signature.PublicKey, x.Vrf.PublicKey) {
			return false, ErrMismatchPublicKeys()
		}
	} else {
		var vals ValSet
		if x.Qc.Header == nil {
			return false, lib.ErrEmptyView()
		}
		vals, err = c.LoadValSet(x.Qc.Header.Height) // REPLICAS: CAPTURE PARTIAL QCs FROM ANY HEIGHT
		if err != nil {
			return false, err
		}
		isPartialQC, err = x.Qc.Check(vals)
		if err != nil {
			return
		}
		if isPartialQC {
			return
		}
		if x.Header.Height != c.Height || x.Qc.Header.Height != c.Height {
			return false, lib.ErrWrongHeight()
		}
		if x.Header.Phase == Propose {
			if err = c.CheckProposal(x.Qc.Proposal); err != nil {
				return
			}
			if len(x.Qc.ProposerKey) != crypto.BLS12381PubKeySize {
				return false, lib.ErrInvalidProposerPubKey()
			}
		} else {
			if !bytes.Equal(x.Qc.ProposalHash, c.HashProposal(c.Proposal)) {
				return false, lib.ErrMismatchProposalHash()
			}
		}
	}
	return
}

func (c *Consensus) CheckReplicaMessage(x *Message) lib.ErrorI {
	if x == nil {
		return ErrEmptyMessage()
	}
	if err := checkSignature(x.Signature, x); err != nil {
		return err
	}
	if x.Qc == nil {
		return lib.ErrEmptyQuorumCertificate()
	}
	if err := x.Qc.Header.Check(c.Height); err != nil {
		return err
	}
	if x.IsPacemakerMessage() {
		return nil
	}
	if x.Qc.Header.Phase == ElectionVote {
		if len(x.Qc.ProposerKey) != crypto.BLS12381PubKeySize {
			return lib.ErrInvalidProposerPubKey()
		}
	} else {
		if !bytes.Equal(x.Qc.ProposalHash, c.HashProposal(c.Proposal)) {
			return lib.ErrMismatchProposalHash()
		}
	}
	return nil
}

func (x *Message) SignBytes() (signBytes []byte) {
	switch {
	case x.IsProposerMessage():
		msg := &Message{
			Header:                 x.Header,
			Vrf:                    x.Vrf,
			HighQc:                 x.HighQc,
			LastDoubleSignEvidence: x.LastDoubleSignEvidence,
			BadProposerEvidence:    x.BadProposerEvidence,
		}
		if x.Qc != nil {
			msg.Qc = &QC{
				Header:       x.Qc.Header,
				ProposalHash: x.Qc.ProposalHash,
				ProposerKey:  x.Qc.ProposerKey,
				Signature:    x.Qc.Signature,
			}
			// ensures messages like hqc doesn't signature fail
			// when proposal is retroactively attached
			if x.Header.Phase == Propose {
				msg.Qc.Proposal = x.Qc.Proposal
			}
		}
		signBytes, _ = lib.Marshal(msg)
	case x.IsReplicaMessage():
		return (&QC{
			Header:       x.Qc.Header,
			ProposalHash: x.Qc.ProposalHash,
			ProposerKey:  x.Qc.ProposerKey,
		}).SignBytes()
	case x.IsPacemakerMessage():
		signBytes, _ = lib.Marshal(&Message{Header: x.Header})
	}
	return
}

func (x *Message) Sign(privateKey crypto.PrivateKeyI) lib.ErrorI {
	x.Signature = new(lib.Signature)
	x.Signature.PublicKey = privateKey.PublicKey().Bytes()
	x.Signature.Signature = privateKey.Sign(x.SignBytes())
	return nil
}

func (x *Message) IsReplicaMessage() bool {
	if x.Qc == nil || x.Qc.Header == nil || x.Header != nil {
		return false
	}
	h := x.Qc.Header
	return h.Phase == ElectionVote || h.Phase == ProposeVote || h.Phase == PrecommitVote
}

func (x *Message) IsProposerMessage() bool {
	h := x.Header
	if h == nil {
		return false
	}
	return h.Phase == Election || h.Phase == Propose || h.Phase == Precommit || h.Phase == Commit
}

func (x *Message) IsPacemakerMessage() bool {
	if x.Qc == nil || x.Qc.Header == nil {
		return false
	}
	return x.Qc.Header.Phase == RoundInterrupt
}

func (x *Message) checkBasic() lib.ErrorI {
	if x == nil {
		return ErrEmptyMessage()
	}
	if err := checkSignature(x.Signature, x); err != nil {
		return err
	}
	if err := x.Header.Check(); err != nil {
		return err
	}
	return nil
}

func checkSignature(signature *lib.Signature, sb lib.SignByte) lib.ErrorI {
	if err := checkSignatureBasic(signature); err != nil {
		return err
	}
	publicKey, err := lib.PublicKeyFromBytes(signature.PublicKey)
	if err != nil {
		return err
	}
	if !publicKey.VerifyBytes(sb.SignBytes(), signature.Signature) {
		return ErrInvalidPartialSignature()
	}
	return nil
}

func checkSignatureBasic(signature *lib.Signature) lib.ErrorI {
	if signature == nil || len(signature.PublicKey) == 0 || len(signature.Signature) == 0 {
		return ErrPartialSignatureEmpty()
	}
	if len(signature.PublicKey) != crypto.BLS12381PubKeySize {
		return ErrInvalidPublicKey()
	}
	if len(signature.Signature) != crypto.BLS12381SignatureSize {
		return ErrInvalidSignatureLength()
	}
	return nil
}
