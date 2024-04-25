package lib

import "bytes"

type ByzantineEvidence struct {
	DSE []*DoubleSignEvidence
	BPE []*BadProposerEvidence
}

type DoubleSignEvidences struct {
	DSE       []*DoubleSignEvidence
	Duplicate map[string]struct{}
}

func NewDSE(dse []*DoubleSignEvidence) DoubleSignEvidences {
	return DoubleSignEvidences{
		DSE:       dse,
		Duplicate: make(map[string]struct{}),
	}
}

func (e *DoubleSignEvidences) Add(
	getValSet func(height uint64) (ValidatorSet, ErrorI),
	getEvByHeight func(height uint64) (*DoubleSigners, ErrorI),
	vs ValidatorSet, ev *DoubleSignEvidence, minimumEvidenceHeight uint64) {
	if !e.isValid(vs, ev, minimumEvidenceHeight) {
		return
	}
	if e.Duplicate == nil {
		e.Duplicate = make(map[string]struct{})
	}
	bz, _ := Marshal(ev)
	key1 := BytesToString(bz)
	bz = ev.FlippedBytes()
	key2 := BytesToString(bz)
	if _, isDuplicate := e.Duplicate[key1]; !isDuplicate {
		if _, isDuplicate = e.Duplicate[key2]; !isDuplicate {
			ev.DoubleSigners = ev.GetBadSigners(getEvByHeight, getValSet)
			if len(ev.DoubleSigners) == 0 {
				return
			}
			e.DSE = append(e.DSE, ev)
			e.Duplicate[key1] = struct{}{}
			e.Duplicate[key2] = struct{}{}
		}
	}
}

func (e *DoubleSignEvidences) HasDoubleSigners() bool {
	for _, ev := range e.DSE {
		if len(ev.DoubleSigners) > 0 {
			return true
		}
	}
	return false
}

func (x DoubleSignEvidence) GetBadSigners(getEvByHeight func(height uint64) (*DoubleSigners, ErrorI),
	getValSet func(height uint64) (ValidatorSet, ErrorI)) (pubKeys [][]byte) {
	ddMap := make(DeDuplicateMap)
	var valSet ValidatorSet
	sig1, sig2 := x.VoteA.Signature, x.VoteB.Signature
	valSet, err := getValSet(x.VoteA.Header.Height)
	if err != nil {
		return
	}
	alreadyCounted, err := getEvByHeight(x.VoteA.Header.Height)
	if err != nil {
		return
	}
	var excluded [][]byte
	if alreadyCounted != nil {
		excluded = alreadyCounted.DoubleSigners
	}
	doubleSigners, err := sig1.GetDoubleSigners(sig2, valSet)
	if err != nil {
		return
	}
OUT:
	for i, ds := range doubleSigners {
		for _, ex := range excluded {
			if bytes.Equal(ex, ds) {
				doubleSigners = append(doubleSigners[:i], doubleSigners[i+1:]...)
				continue OUT
			}
		}
	}
	ddMap.Add(x.VoteA.Header, doubleSigners, pubKeys)
	return
}

func (e DoubleSignEvidences) IsValid(vs ValidatorSet, minimumEvidenceHeight uint64) (ok bool) {
	for _, ev := range e.DSE {
		if !e.isValid(vs, ev, minimumEvidenceHeight) {
			return
		}
	}
	return true
}

func (e DoubleSignEvidences) isValid(vs ValidatorSet, ev *DoubleSignEvidence, minimumEvidenceHeight uint64) (ok bool) {
	if err := ev.CheckBasic(); err != nil {
		return
	}
	if err := ev.Check(vs, minimumEvidenceHeight); err != nil {
		return
	}
	if equals := ev.VoteA.Equals(ev.VoteB); equals {
		return
	}
	return true
}

func (e DoubleSignEvidences) Equals(b DoubleSignEvidences) bool {
	bz1, _ := Marshal(e)
	bz2, _ := Marshal(b)
	return bytes.Equal(bz1, bz2)
}

type BadProposerEvidences struct {
	BPE       []*BadProposerEvidence
	Duplicate map[string]struct{}
}

func NewBPE(bpe []*BadProposerEvidence) BadProposerEvidences {
	return BadProposerEvidences{
		BPE:       bpe,
		Duplicate: make(map[string]struct{}),
	}
}

func (e *BadProposerEvidences) Add(trueLeader []byte, height uint64, vs ValidatorSet, ev *BadProposerEvidence) {
	if !e.isValid(trueLeader, height, vs, ev) {
		return
	}
	if e.Duplicate == nil {
		e.Duplicate = make(map[string]struct{})
	}
	bz, _ := Marshal(ev.ElectionVoteQc.Header)
	key := BytesToString(bz) + BytesToString(ev.ElectionVoteQc.ProposerKey)
	if _, isDuplicate := e.Duplicate[key]; !isDuplicate {
		e.BPE = append(e.BPE, ev)
		e.Duplicate[key] = struct{}{}
	}
}

func (e BadProposerEvidences) GetBadProposers() (pubKeys [][]byte) {
	ddMap := make(DeDuplicateMap)
	for _, bp := range e.BPE {
		ddMap.Add(bp.ElectionVoteQc.Header, [][]byte{bp.ElectionVoteQc.ProposerKey}, pubKeys)
	}
	return
}

func (e BadProposerEvidences) IsValid(trueLeader []byte, height uint64, set ValidatorSet) (ok bool) {
	for _, ev := range e.BPE {
		if !e.isValid(trueLeader, height, set, ev) {
			return
		}
	}
	return true
}

func (e BadProposerEvidences) isValid(trueLeader []byte, height uint64, vs ValidatorSet, ev *BadProposerEvidence) (ok bool) {
	if ev == nil {
		return
	}
	isPartialQC, err := ev.ElectionVoteQc.Check(height, vs)
	if isPartialQC || err != nil || ev.ElectionVoteQc.Header.Phase != Phase_ELECTION_VOTE {
		return
	}
	if bytes.Equal(ev.ElectionVoteQc.ProposerKey, trueLeader) {
		return
	}
	return true
}

type DeDuplicateMap map[string]map[string]struct{}

func (d DeDuplicateMap) Add(view *View, tryAdd, result [][]byte) {
	viewBytes, err := Marshal(view)
	if err != nil {
		return
	}
	key1 := BytesToString(viewBytes)
	if _, ok := d[key1]; !ok {
		d[key1] = make(map[string]struct{})
	}
	for _, try := range tryAdd {
		key2 := BytesToString(try)
		if _, ok := d[key1][key2]; ok {
			continue
		} else {
			d[key1][key2] = struct{}{}
			result = append(result, try)
		}
	}
}
