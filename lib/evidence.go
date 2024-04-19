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

func (e *DoubleSignEvidences) Add(height uint64, vs, lastVS ValidatorSet, ev *DoubleSignEvidence) {
	if !e.isValid(height, vs, lastVS, ev) {
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
			e.DSE = append(e.DSE, ev)
			e.Duplicate[key1] = struct{}{}
			e.Duplicate[key2] = struct{}{}
		}
	}
}

func (e DoubleSignEvidences) GetDoubleSigners(height uint64, vs, lastVS ValidatorSet) (pubKeys [][]byte) {
	ddMap := make(DeDuplicateMap)
	for _, ev := range e.DSE {
		var valSet ValidatorSet
		sig1, sig2 := ev.VoteA.Signature, ev.VoteB.Signature
		if ev.VoteA.Header.Height == height {
			valSet = vs
		} else {
			valSet = lastVS
		}
		doubleSigners, err := sig1.GetDoubleSigners(sig2, valSet)
		if err != nil {
			return
		}
		ddMap.Add(ev.VoteA.Header, doubleSigners, pubKeys)
	}
	return
}

func (e DoubleSignEvidences) IsValid(height uint64, vs, lastVS ValidatorSet) (ok bool) {
	for _, ev := range e.DSE {
		if !e.isValid(height, vs, lastVS, ev) {
			return
		}
	}
	return true
}

func (e DoubleSignEvidences) isValid(height uint64, vs, lastVS ValidatorSet, ev *DoubleSignEvidence) (ok bool) {
	var valSet ValidatorSet
	if err := ev.CheckBasic(height); err != nil {
		return
	}
	if ev.VoteA.Header.Height == height {
		valSet = vs
	} else {
		valSet = lastVS
	}
	if err := ev.Check(valSet); err != nil {
		return
	}
	if equals := ev.VoteA.Equals(ev.VoteB); equals {
		return
	}
	return true
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
