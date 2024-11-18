package bft

import (
	"bytes"
	"github.com/canopy-network/canopy/lib"
	"slices"
)

// ByzantineEvidence represents a collection of evidence that supports byzantine behavior during the BFT lifecycle
// this Evidence is circulated to the Leader of a Round and is processed in the execution of Reward Transactions
type ByzantineEvidence struct {
	DSE DoubleSignEvidences  // Evidence of `DoubleSigning`: Signing two different messages for the same View is against protocol rules (breaks protocol safety)
	BPE BadProposerEvidences // Evidence of `BadProposer`: Faulty leaders are punished as their lack of participation halts the BFT process until the next Round
}

// ValidateByzantineEvidence() ensures the DoubleSigners in the Proposal are supported by the ByzantineEvidence
func (b *BFT) ValidateByzantineEvidence(slashRecipients *lib.SlashRecipients, be *ByzantineEvidence) lib.ErrorI {
	if slashRecipients == nil {
		return nil
	}
	if slashRecipients.DoubleSigners != nil {
		// locally generate a Double Signers list from the provided evidence
		doubleSigners, err := b.ProcessDSE(be.DSE.Evidence...)
		if err != nil {
			return err
		}
		// this validation ensures that the Meta.DoubleSigners is justified, but there may be additional evidence included without any error
		for _, ds := range slashRecipients.DoubleSigners {
			if ds == nil {
				return lib.ErrInvalidEvidence()
			}
			// check if the Double Signer in the Proposal is within our locally generated Double Signers list
			if !slices.ContainsFunc(doubleSigners, func(signer *lib.DoubleSigner) bool {
				if signer == nil {
					return false
				}
				if !bytes.Equal(ds.PubKey, signer.PubKey) {
					return false
				}
				// validate each height slash per double signer is also justified
				for _, height := range ds.Heights {
					if !slices.Contains(signer.Heights, height) {
						return false
					}
				}
				return true
			}) {
				return lib.ErrMismatchEvidenceAndHeader()
			}
		}
	}
	if slashRecipients.BadProposers != nil {
		// generate a Bad Proposers list from the provided evidence
		badProposers, err := b.ProcessBPE(be.BPE.Evidence...)
		if err != nil {
			return err
		}
		// this validation ensures that the bad proposers are justified, but there may be additional evidence included without any error
		for _, bp := range slashRecipients.BadProposers {
			// check if the Bad Proposer in the Proposal is within our locally generated Bad Proposers list
			if !slices.ContainsFunc(badProposers, func(badProposer []byte) bool {
				return bytes.Equal(badProposer, bp)
			}) {
				return lib.ErrMismatchEvidenceAndHeader()
			}
		}
	}
	return nil
}

// DOUBLE SIGN EVIDENCE

// NewDSE() creates a list of DoubleSignEvidences with a built-in DeDuplicator
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

// ProcessDSE() validates each piece of double sign evidence and returns a list of double signers
func (b *BFT) ProcessDSE(dse ...*DoubleSignEvidence) (results []*lib.DoubleSigner, e lib.ErrorI) {
	results = make([]*lib.DoubleSigner, 0)
	for _, x := range dse {
		// sanity check the evidence
		if err := x.CheckBasic(); err != nil {
			return nil, err
		}
		// load the Validator set for this Committee at that height
		committeeHeight := x.VoteA.Header.CanopyHeight
		vs, err := b.LoadCommittee(b.CommitteeId, committeeHeight)
		if err != nil {
			return nil, err
		}
		// ensure the evidence isn't expired
		minEvidenceHeight, err := b.LoadMinimumEvidenceHeight()
		if err != nil {
			return nil, err
		}
		// validate the piece of evidence
		if err = x.Check(vs, b.View, minEvidenceHeight); err != nil {
			return nil, err
		}
		// if the votes are identical - it's not a double sign...
		if bytes.Equal(x.VoteB.SignBytes(), x.VoteA.SignBytes()) {
			return nil, lib.ErrInvalidEvidence() // same payloads
		}
		// take the signatures from the two
		sig1, sig2 := x.VoteA.Signature, x.VoteB.Signature
		doubleSigners, err := sig1.GetDoubleSigners(sig2, vs)
		if err != nil {
			return nil, err
		}
		// the evidence may include double signers who were already slashed for that height
		// if so, ignore those double signers but still process the rest of the bad actors
	out:
		for _, pubKey := range doubleSigners {
			if b.IsValidDoubleSigner(committeeHeight, pubKey) {
				for i, doubleSigner := range results {
					if bytes.Equal(doubleSigner.PubKey, pubKey) {
						results[i].AddHeight(committeeHeight)
						continue out
					}
				}
				results = append(results, &lib.DoubleSigner{
					PubKey:  pubKey,
					Heights: []uint64{committeeHeight},
				})
			}
		}
	}
	return
}

// AddDSE() validates and adds new DoubleSign Evidence to a list of DoubleSignEvidences
func (b *BFT) AddDSE(e *DoubleSignEvidences, ev *DoubleSignEvidence) (err lib.ErrorI) {
	// basic sanity checks for the evidence
	if err = ev.CheckBasic(); err != nil {
		return
	}
	// nullify the block and results as they are unnecessary bloat in the message for this purpose
	ev.VoteA.Block, ev.VoteA.Results = nil, nil
	ev.VoteB.Block, ev.VoteB.Results = nil, nil
	// process the Double Sign Evidence and save the double signers
	badSigners, err := b.ProcessDSE(ev)
	if err != nil {
		return err
	}
	// ignore if there are no bad actors
	if len(badSigners) == 0 {
		return lib.ErrInvalidEvidence()
	}
	// de duplicate the evidence
	if e.DeDuplicator == nil {
		e.DeDuplicator = make(map[string]bool)
	}
	// NOTE: this de-duplication is only good for 'accidental' duplication
	// evidence could be replayed - but it would always only result in
	// 1 slash per signer per Canopy height
	bz, _ := lib.Marshal(ev)
	key1 := lib.BytesToString(bz)
	if _, isDuplicate := e.DeDuplicator[key1]; isDuplicate {
		return
	}
	e.Evidence = append(e.Evidence, ev)
	e.DeDuplicator[key1] = true
	return
}

// GetLocalDSE() returns the double sign evidences collected by the local node
func (b *BFT) GetLocalDSE() DoubleSignEvidences {
	dse := NewDSE()
	// by partial QC: a byzantine Leader sent a 'non +2/3 quorum certificate'
	// and the node holds a correct Quorum Certificate for the same View
	b.addDSEByPartialQC(&dse)
	// by candidate: a good samaritan Leader Candidate node receives ELECTION votes from Replicas
	// that also voted for the 'true Leader' the candidate now holds equivocating signatures by Replicas
	// for the same View
	b.addDSEByCandidate(&dse)
	return dse
}

// CheckBasic() executes basic sanity checks on the DoubleSign Evidence
// It's important to note that DoubleSign evidence may be processed for any height
// thus it's never validated against 'current height'
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

// Check() validates the double sign evidence
func (x *DoubleSignEvidence) Check(vs lib.ValidatorSet, view *lib.View, minimumEvidenceHeight uint64) lib.ErrorI {
	// can't be too old
	if x.VoteA.Header.CanopyHeight < minimumEvidenceHeight {
		return lib.ErrEvidenceTooOld()
	}
	// ensure large payloads are empty as they are unnecessary for this message
	if x.VoteA.Block != nil || x.VoteB.Block != nil {
		return lib.ErrExpectedMaxBlockSize()
	}
	if x.VoteA.Results != nil || x.VoteB.Results != nil {
		return lib.ErrNonNilCertResults()
	}
	// should be a valid QC for the committee
	// NOTE: CheckBasic() purposefully doesn't return errors on partial QCs
	if _, err := x.VoteA.Check(vs, 0, view, false); err != nil {
		return err
	}
	if _, err := x.VoteB.Check(vs, 0, view, false); err != nil {
		return err
	}
	// ensure it's the same height
	if !x.VoteA.Header.Equals(x.VoteB.Header) {
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

// AddPartialQC() saves a non-majority Quorum Certificate which is a big hint of faulty behavior
func (b *BFT) AddPartialQC(m *Message) (err lib.ErrorI) {
	bz, err := lib.Marshal(m.Qc)
	if err != nil {
		return
	}
	b.PartialQCs[lib.BytesToString(bz)] = m.Qc
	return
}

// addDSEByPartialQC() attempts to convert all partial QCs to DoubleSignEvidence by finding a saved conflicting valid QC
func (b *BFT) addDSEByPartialQC(dse *DoubleSignEvidences) {
	// REPLICA with two proposer messages for same (H,R,P) - the partial is the malicious one
	for _, pQC := range b.PartialQCs {
		evidenceHeight := pQC.Header.Height
		if evidenceHeight == b.Height {
			// get the round of the partial QC to try to find a conflicting majority QC
			roundProposal := b.Proposals[pQC.Header.Round]
			if roundProposal == nil {
				continue
			}
			// try to find a conflicting QC
			// NOTE: proposals with conflicting QC is 1 phase above as it's used by the leader as Justification
			proposal, found := roundProposal[phaseToString(pQC.Header.Phase+1)]
			if !found {
				continue
			}
			// add the double sign evidence to the list
			if err := b.AddDSE(dse, &DoubleSignEvidence{
				VoteA: proposal[0].Qc, // if both a partial and full exists
				VoteB: pQC,
			}); err != nil {
				b.log.Error(err.Error())
			}
		} else { // this partial QC is historical
			// historically can only process precommit vote as the other non Commit QCs are pruned
			if pQC.Header.Phase != PrecommitVote {
				continue
			}
			// Load the certificate that contains the competing QC
			certificate, err := b.LoadCertificate(b.CommitteeId, evidenceHeight)
			if err != nil {
				continue
			}
			// add the double sign evidence to the list
			if err = b.AddDSE(dse, &DoubleSignEvidence{
				VoteA: certificate, // if both a partial and full exists
				VoteB: pQC,
			}); err != nil {
				b.log.Error(err.Error())
			}
		}
	}
}

// addDSEByCandidate() node looks through any ELECTION-VOTES they received to see if any signers conflict with signers of the true Leader
func (b *BFT) addDSEByCandidate(dse *DoubleSignEvidences) {
	// ELECTION CANDIDATE exposing double sign election
	if !b.SelfIsProposer() {
		// for each Round until present
		for r := uint64(0); r <= b.Round; r++ {
			// get votes for round
			rvs := b.Votes[r]
			if rvs == nil {
				continue
			}
			// get votes for Election Vote phase
			ps := phaseToString(ElectionVote)
			ev := rvs[ps]
			if ev == nil {
				continue
			}
			// get the true Leaders' messages for this same round
			roundProposal := b.Proposals[r]
			if roundProposal == nil {
				continue
			}
			// specifically the Propose message which contains the ElectionVote justification signatures
			proposal, found := roundProposal[phaseToString(Propose)]
			if !found {
				continue
			}
			// for each election vote payload received (likely only 1 payload)
			for _, voteSet := range ev {
				// if the vote exists and the QC is not empty
				if voteSet.Vote != nil && voteSet.Vote.Qc != nil {
					// aggregate the signers of this vote
					as, e := voteSet.multiKey.AggregateSignatures()
					if e != nil {
						continue
					}
					// build into an ElectionQC
					qc := &QC{
						Header:      voteSet.Vote.Qc.Header,
						ProposerKey: voteSet.Vote.Qc.ProposerKey,
						Signature: &lib.AggregateSignature{
							Signature: as,
							Bitmap:    voteSet.multiKey.Bitmap(),
						},
					}
					// attempt to add it as DoubleSignEvidence
					// (will be rejected if there are no conflicting signers)
					if err := b.AddDSE(dse, &DoubleSignEvidence{
						VoteA: proposal[0].Qc,
						VoteB: qc,
					}); err != nil {
						b.log.Warn(err.Error())
					}
				}
			}
		}
	}
}

// BAD PROPOSER EVIDENCE BELOW

// NewBPE() creates a list of BadProposerEvidences with a builtin de-duplicator
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

// ProcessBPE() validates each piece of bad proposer evidence and returns a list of bad proposers
func (b *BFT) ProcessBPE(x ...*BadProposerEvidence) (badProposers [][]byte, err lib.ErrorI) {
	for _, ev := range x {
		cert, e := b.LoadCertificate(b.CommitteeId, b.Height-1)
		if e != nil {
			return nil, e
		}
		// validate the evidence
		if !ev.Check(cert.ProposerKey, b.View, b.ValidatorSet) {
			return nil, lib.ErrInvalidEvidence()
		}
		// validate the evidence height
		if ev.ElectionVoteQc.Header.Height != b.Height-1 {
			return nil, lib.ErrWrongHeight()
		}
	}
	// de duplicate the list
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

// GetLocalBPE() generates local BPE from any 'proposers' who did not complete their task of leading the round
func (b *BFT) GetLocalBPE() BadProposerEvidences {
	e := NewBPE()
	for r := uint64(0); r < b.Round; r++ {
		if msg := b.getProposal(r, ProposeVote); msg != nil && msg.Qc != nil {
			if err := b.AddBPE(&e, &BadProposerEvidence{ElectionVoteQc: msg.Qc}, true); err != nil {
				b.log.Error(err.Error())
			}
		}
	}
	return e
}

// AddBPE() attempts to add a piece of bad proposer evidence to the list
func (b *BFT) AddBPE(bpe *BadProposerEvidences, ev *BadProposerEvidence, local bool) lib.ErrorI {
	// sanity check the evidence
	if !ev.Check(b.ProposerKey, b.View, b.ValidatorSet) {
		return lib.ErrInvalidEvidence()
	}
	// validate the evidence height
	validationHeight := b.Height - 1
	// if it's locally generated evidence, then it's at the COMMIT_PROCESS phase and should be the same height
	if local {
		validationHeight = b.Height
	}
	// validate the evidence height
	if ev.ElectionVoteQc.Header.Height != validationHeight {
		return lib.ErrWrongHeight()
	}
	// prepare de duplicator
	if bpe.DeDuplicator == nil {
		bpe.DeDuplicator = make(map[string]bool)
	}
	// add evidence to list and de-duplicator
	bz, _ := lib.Marshal(ev.ElectionVoteQc.Header)
	key := lib.BytesToString(bz) + lib.BytesToString(ev.ElectionVoteQc.ProposerKey)
	if _, isDuplicate := bpe.DeDuplicator[key]; !isDuplicate {
		bpe.Evidence = append(bpe.Evidence, ev)
		bpe.DeDuplicator[key] = true
	}
	return nil
}

// Check() performs a full validation of bad proposer evidence
func (x *BadProposerEvidence) Check(trueLeader []byte, view *lib.View, vs lib.ValidatorSet) (ok bool) {
	if x == nil {
		return
	}
	// ensure this is a valid election vote quorum certificate
	isPartialQC, err := x.ElectionVoteQc.Check(vs, 0, view, false)
	if isPartialQC || err != nil || x.ElectionVoteQc.Header.Phase != lib.Phase_ELECTION_VOTE {
		return
	}
	// the true leader cannot be a 'bad proposer' even if was faulty before
	// This is because non-consensus participants cannot determine the exact Round when the Consensus was completed
	// Without this check, a valid ElectionQC for the true leader could be wrongly used as evidence
	if bytes.Equal(x.ElectionVoteQc.ProposerKey, trueLeader) {
		return
	}
	return true
}
