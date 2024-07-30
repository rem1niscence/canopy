package bft

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"reflect"
)

type ByzantineEvidence struct {
	DSE DoubleSignEvidences
	BPE BadProposerEvidences
}

func (c *Consensus) ValidateByzantineEvidence(x *lib.BlockHeader, be *ByzantineEvidence) lib.ErrorI {
	if be == nil {
		return nil
	}
	if x.DoubleSigners != nil {
		doubleSigners, err := c.ProcessDSE(be.DSE.Evidence...)
		if err != nil {
			return err
		}
		// check evidence amounts to the same double signer conclusion
		if len(doubleSigners) != len(x.DoubleSigners) {
			return lib.ErrInvalidEvidence()
		}
		for address, heights := range doubleSigners {
			checkHeights, ok := x.DoubleSigners[address]
			if !ok {
				return lib.ErrInvalidEvidence()
			}
			if !reflect.DeepEqual(heights.Heights, checkHeights.Heights) {
				return lib.ErrInvalidEvidence()
			}
		}
	}
	if x.BadProposers != nil {
		badProposers, err := c.ProcessBPE(be.BPE.Evidence...)
		if err != nil {
			return err
		}
		// check evidence amounts to the same bad proposer conclusion
		if len(badProposers) != len(x.BadProposers) {
			return lib.ErrMismatchBadProducerCount()
		}
		for i, bp := range x.BadProposers {
			if !bytes.Equal(badProposers[i], bp) {
				return lib.ErrMismatchEvidenceAndHeader()
			}
		}
	}
	return nil
}

// DOUBLE SIGN EVIDENCE

func NewDSE(dse ...[]*DoubleSignEvidence) DoubleSignEvidences {
	ds := make([]*DoubleSignEvidence, 0)
	if dse != nil {
		ds = dse[0]
	}
	return DoubleSignEvidences{
		Evidence:     ds,
		DeDuplicator: make(map[string]bool),
	}
}

func (c *Consensus) ProcessDSE(dse ...*DoubleSignEvidence) (results map[string]*lib.DoubleSignHeights, e lib.ErrorI) {
	results = make(map[string]*lib.DoubleSignHeights)
	for _, x := range dse {
		if err := x.CheckBasic(); err != nil {
			return nil, err
		}
		vs, err := c.LoadValSet(x.VoteA.Header.Height)
		if err != nil {
			return nil, err
		}
		minEvidenceHeight, err := c.LoadMinimumEvidenceHeight()
		if err != nil {
			return nil, err
		}
		if err = x.Check(vs, minEvidenceHeight); err != nil {
			return nil, err
		}
		if equals := x.VoteA.Equals(x.VoteB); equals {
			return nil, err
		}
		sig1, sig2, height := x.VoteA.Signature, x.VoteB.Signature, x.VoteA.Header.Height
		valSet, err := c.LoadValSet(x.VoteA.Header.Height)
		if err != nil {
			return nil, err
		}
		doubleSigners, err := sig1.GetDoubleSigners(sig2, valSet)
		if err != nil {
			return nil, err
		}
		for _, ds := range doubleSigners {
			if c.IsValidDoubleSigner(height, ds) {
				addressKey := lib.BytesToString(ds)
				heights, ok := results[addressKey]
				if !ok {
					heights = &lib.DoubleSignHeights{Heights: make(map[uint64]bool)}
				}
				heights.Heights[height] = true
				results[addressKey] = heights
			}
		}
	}
	return
}

func (c *Consensus) AddDSE(e *DoubleSignEvidences, ev *DoubleSignEvidence) (err lib.ErrorI) {
	if err = ev.CheckBasic(); err != nil {
		return
	}
	badSigners, err := c.ProcessDSE(ev)
	if err != nil {
		return err
	}
	if len(badSigners) == 0 {
		return lib.ErrInvalidEvidence()
	}
	if e.DeDuplicator == nil {
		e.DeDuplicator = make(map[string]bool)
	}
	bz, _ := lib.Marshal(ev)
	key1 := lib.BytesToString(bz)
	if _, isDuplicate := e.DeDuplicator[key1]; isDuplicate {
		return
	}
	e.Evidence = append(e.Evidence, ev)
	e.DeDuplicator[key1] = true
	return
}

func (c *Consensus) GetDSE() DoubleSignEvidences {
	dse := NewDSE()
	c.addDSEByPartialQC(&dse)
	c.addDSEByCandidate(&dse)
	return dse
}

func (x *DoubleSignEvidence) CheckBasic() lib.ErrorI {
	if x == nil {
		return lib.ErrEmptyEvidence()
	}
	if x.VoteA == nil || x.VoteB == nil || x.VoteA.Header == nil || x.VoteB.Header == nil {
		return lib.ErrEmptyQuorumCertificate()
	}
	if !x.VoteA.Header.Equals(x.VoteB.Header) {
		return lib.ErrMismatchEvidenceAndHeader()
	}
	if x.VoteA.Header.Round >= lib.MaxRound {
		return lib.ErrWrongRound()
	}
	return nil
}

func (x *DoubleSignEvidence) Check(vs lib.ValidatorSet, minimumEvidenceHeight uint64) lib.ErrorI {
	if x.VoteA.Header.Height < minimumEvidenceHeight {
		return lib.ErrEvidenceTooOld()
	}
	if _, err := x.VoteA.Check(vs); err != nil {
		return err
	}
	if _, err := x.VoteB.Check(vs); err != nil {
		return err
	}
	if !x.VoteA.Header.Equals(x.VoteB.Header) || x.VoteA.Equals(x.VoteB) {
		return lib.ErrInvalidEvidence() // different heights
	}
	if bytes.Equal(x.VoteB.SignBytes(), x.VoteA.SignBytes()) {
		return lib.ErrInvalidEvidence() // same payloads
	}
	return nil
}

// PartialQCs are saved < +2/3 majority Quorum Certificates received by a malicious or faulty proposer
// PartialQCs are not Double Sign evidence until coupled with a full QC of the same View
// Correct replicas save partial QCs to check for double sign evidence to send to the next proposer during the ElectionVote phase
type PartialQCs map[string]*QC // [ PayloadHash ] -> Partial QC

func (c *Consensus) AddPartialQC(m *Message) (err lib.ErrorI) {
	bz, err := lib.Marshal(m.Qc)
	if err != nil {
		return
	}
	c.PartialQCs[lib.BytesToString(bz)] = m.Qc
	return
}

func (c *Consensus) addDSEByPartialQC(dse *DoubleSignEvidences) {
	for _, pQC := range c.PartialQCs { // REPLICA with two proposer messages for same (H,R,P) - the partial is the malicious one
		evidenceHeight := pQC.Header.Height
		if evidenceHeight == c.Height {
			roundProposal := c.Proposals[pQC.Header.Round]
			if roundProposal == nil {
				continue
			}
			proposal, found := roundProposal[phaseToString(pQC.Header.Phase+1)] // proposals with competing QC is 1 phase above
			if !found {
				continue
			}
			if err := c.AddDSE(dse, &DoubleSignEvidence{
				VoteA: proposal[0].Qc, // if both a partial and full exists
				VoteB: pQC,
			}); err != nil {
				c.log.Error(err.Error())
			}
		} else {
			if pQC.Header.Phase != PrecommitVote {
				continue // historically can only process precommit vote
			}
			certificate, err := c.LoadCertificate(evidenceHeight)
			if err != nil {
				continue
			}
			if err = c.AddDSE(dse, &DoubleSignEvidence{
				VoteA: certificate, // if both a partial and full exists
				VoteB: pQC,
			}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
}

func (c *Consensus) addDSEByCandidate(dse *DoubleSignEvidences) {
	if !c.SelfIsProposer() { // ELECTION CANDIDATE exposing double sign election
		for r := uint64(0); r <= c.Round; r++ {
			rvs := c.Votes[r]
			if rvs == nil {
				continue
			}
			ps := phaseToString(ElectionVote)
			ev := rvs[ps]
			if ev == nil {
				continue
			}
			roundProposal := c.Proposals[r]
			if roundProposal == nil {
				continue
			}
			proposal, found := roundProposal[phaseToString(Propose)]
			if !found {
				continue
			}
			for _, voteSet := range ev {
				if voteSet.Vote != nil && voteSet.Vote.Qc != nil {
					as, e := voteSet.multiKey.AggregateSignatures()
					if e != nil {
						continue
					}
					qc := &QC{
						Header:      voteSet.Vote.Qc.Header,
						ProposerKey: voteSet.Vote.Qc.ProposerKey,
						Signature: &lib.AggregateSignature{
							Signature: as,
							Bitmap:    voteSet.multiKey.Bitmap(),
						},
					}
					if err := c.AddDSE(dse, &DoubleSignEvidence{
						VoteA: proposal[0].Qc,
						VoteB: qc,
					}); err != nil {
						c.log.Error(err.Error())
					}
				}
			}
		}
	}
}

// BAD PROPOSER EVIDENCE

func NewBPE(bpe ...[]*BadProposerEvidence) BadProposerEvidences {
	bp := make([]*BadProposerEvidence, 0)
	if bpe != nil {
		bp = bpe[0]
	}
	return BadProposerEvidences{
		Evidence:     bp,
		DeDuplicator: make(map[string]bool),
	}
}

func (c *Consensus) ProcessBPE(x ...*BadProposerEvidence) (badProposers [][]byte, err lib.ErrorI) {
	for _, ev := range x {
		if !ev.Check(nil, c.Height, c.ValidatorSet) {
			return nil, lib.ErrInvalidEvidence()
		}
	}
	dedupe := make(map[string]struct{})
	for _, bp := range x {
		proposerKey := lib.BytesToString(bp.ElectionVoteQc.ProposerKey)
		if _, exists := dedupe[proposerKey]; exists {
			continue
		}
		badProposers, dedupe[proposerKey] = append(badProposers, bp.ElectionVoteQc.ProposerKey), struct{}{}
	}
	return
}

func (c *Consensus) AddBPE(bpe *BadProposerEvidences, ev *BadProposerEvidence) lib.ErrorI {
	if !ev.Check(c.ProposerKey, c.Height, c.ValidatorSet) {
		return lib.ErrInvalidEvidence()
	}
	if bpe.DeDuplicator == nil {
		bpe.DeDuplicator = make(map[string]bool)
	}
	bz, _ := lib.Marshal(ev.ElectionVoteQc.Header)
	key := lib.BytesToString(bz) + lib.BytesToString(ev.ElectionVoteQc.ProposerKey)
	if _, isDuplicate := bpe.DeDuplicator[key]; !isDuplicate {
		bpe.Evidence = append(bpe.Evidence, ev)
		bpe.DeDuplicator[key] = true
	}
	return nil
}

func (c *Consensus) GetBPE() BadProposerEvidences {
	e := NewBPE()
	for r := uint64(0); r < c.Round; r++ {
		if msg := c.getProposal(r, ProposeVote); msg != nil && msg.Qc != nil {
			if err := c.AddBPE(&e, &BadProposerEvidence{ElectionVoteQc: msg.Qc}); err != nil {
				c.log.Error(err.Error())
			}
		}
	}
	return e
}

func (x *BadProposerEvidence) Check(trueLeader []byte, height uint64, vs lib.ValidatorSet) (ok bool) {
	if x == nil {
		return
	}
	isPartialQC, err := x.ElectionVoteQc.Check(vs, height)
	if isPartialQC || err != nil || x.ElectionVoteQc.Header.Phase != lib.Phase_ELECTION_VOTE {
		return
	}
	if bytes.Equal(x.ElectionVoteQc.ProposerKey, trueLeader) {
		return
	}
	return true
}
