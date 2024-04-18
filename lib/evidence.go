package lib

import "bytes"

type ByzantineEvidence struct {
	DSE DoubleSignEvidences
	BPE BadProposerEvidences
}

type DoubleSignEvidences []*DoubleSignEvidence

func (e *DoubleSignEvidences) Add(height uint64, vs, lastVS ValidatorSet, evidence *DoubleSignEvidence) {
	var valSet ValidatorSet
	if err := evidence.CheckBasic(height); err != nil {
		return
	}
	if evidence.VoteA.Header.Height == height {
		valSet = vs
	} else {
		valSet = lastVS
	}
	if err := evidence.Check(valSet); err != nil {
		return
	}
	if equals := evidence.VoteA.Equals(evidence.VoteB); equals {
		return
	}
	duplicate := make(map[string]struct{})
	for _, ev := range *e {
		bz, _ := Marshal(ev)
		key := BytesToString(bz)
		duplicate[key] = struct{}{}
		bz = ev.FlippedBytes()
		key = BytesToString(bz)
		duplicate[key] = struct{}{}
	}
	bz, _ := Marshal(evidence)
	key1 := BytesToString(bz)
	bz = evidence.FlippedBytes()
	key2 := BytesToString(bz)
	if _, isDuplicate := duplicate[key1]; !isDuplicate {
		if _, isDuplicate = duplicate[key2]; !isDuplicate {
			*e = append(*e, evidence)
		}
	}
}

func (e *DoubleSignEvidences) FilterBad(height uint64, vs, lastVS ValidatorSet) ErrorI { // TODO de-dup
	if e == nil {
		return ErrEmptyEvidence()
	}
	var valSet ValidatorSet
	var goodEvidences DoubleSignEvidences
	for _, evidence := range *e {
		if evidence == nil {
			continue
		}
		if err := evidence.CheckBasic(height); err != nil {
			continue
		}
		if evidence.VoteA.Header.Height == height {
			valSet = vs
		} else {
			valSet = lastVS
		}
		if err := evidence.Check(valSet); err != nil {
			continue
		}
		if equals := evidence.VoteA.Equals(evidence.VoteB); equals {
			continue
		}
		goodEvidences = append(goodEvidences, evidence)
	}
	*e = goodEvidences
	return nil
}

func (e DoubleSignEvidences) GetDoubleSigners(height uint64, vs, lastVS ValidatorSet) (pubKeys [][]byte, error ErrorI) {
	if err := e.FilterBad(height, vs, lastVS); err != nil {
		return nil, err
	}
	if len(e) == 0 {
		return
	}
	// one infraction per view
	deDupMap := make(map[string]map[string]struct{}) // view -> map[pubkey]
	for _, evidence := range e {
		valSet, h := ValidatorSet{}, evidence.VoteA.Header.Height
		viewBytes, err := Marshal(evidence.VoteA.Header)
		if err != nil {
			return nil, err
		}
		viewKey := BytesToString(viewBytes)
		if _, ok := deDupMap[viewKey]; !ok {
			deDupMap[viewKey] = make(map[string]struct{})
		}
		aggSig1 := evidence.VoteA.Signature
		aggSig2 := evidence.VoteB.Signature
		if h == height {
			valSet = vs
		} else {
			valSet = lastVS
		}
		doubleSigners, err := aggSig1.GetDoubleSigners(aggSig2, valSet)
		if err != nil {
			return nil, err
		}
		for _, ds := range doubleSigners {
			if _, ok := deDupMap[viewKey][BytesToString(ds)]; ok {
				continue
			} else {
				// add to de-dup map
				m := deDupMap[viewKey]
				m[BytesToString(ds)] = struct{}{}
				deDupMap[viewKey] = m
				// add to infraction
				pubKeys = append(pubKeys, ds)
			}
		}
	}
	return
}

type BadProposerEvidences []*BadProposerEvidence

func (bpe *BadProposerEvidences) Add(expectedLeader []byte, height uint64, vs ValidatorSet, evidence *BadProposerEvidence) {
	if evidence == nil {
		return
	}
	isPartialQC, err := evidence.ElectionVoteQc.Check(&View{Height: height}, vs)
	if err != nil {
		return
	}
	if isPartialQC {
		return
	}
	if bytes.Equal(evidence.ElectionVoteQc.LeaderPublicKey, expectedLeader) {
		return
	}
	duplicate := make(map[string]struct{})
	for _, e := range *bpe {
		bz, _ := Marshal(e.ElectionVoteQc.Header)
		key := BytesToString(bz) + BytesToString(e.ElectionVoteQc.LeaderPublicKey)
		duplicate[key] = struct{}{}
	}
	bz, _ := Marshal(evidence.ElectionVoteQc.Header)
	key := BytesToString(bz) + BytesToString(evidence.ElectionVoteQc.LeaderPublicKey)
	if _, isDuplicate := duplicate[key]; !isDuplicate {
		*bpe = append(*bpe, evidence)
	}
}

func (bpe *BadProposerEvidences) FilterBad(expectedLeader []byte, height uint64, vs ValidatorSet) ErrorI {
	if bpe == nil {
		return ErrEmptyEvidence()
	}
	var goodEvidence BadProposerEvidences
	for _, evidence := range *bpe {
		if evidence == nil {
			continue
		}
		isPartialQC, err := evidence.ElectionVoteQc.Check(&View{Height: height}, vs)
		if err != nil {
			continue
		}
		if isPartialQC {
			continue
		}
		if bytes.Equal(evidence.ElectionVoteQc.LeaderPublicKey, expectedLeader) {
			continue
		}
		goodEvidence = append(goodEvidence, evidence)
	}
	*bpe = goodEvidence
	return nil
}

func (bpe BadProposerEvidences) GetBadProposers(expectedLeader []byte, height uint64, vs ValidatorSet) (pubKeys [][]byte, error ErrorI) {
	if err := bpe.FilterBad(expectedLeader, height, vs); err != nil {
		return nil, err
	}
	deDupMap := make(map[string]map[string]struct{}) // view -> map[pubkey]
	for _, bp := range bpe {
		isPartialQC, err := bp.ElectionVoteQc.Check(&View{Height: height}, vs)
		if err != nil {
			continue // log error?
		}
		if isPartialQC {
			continue
		}
		if bp.ElectionVoteQc.Header.Phase != Phase_ELECTION_VOTE {
			continue
		}
		viewBytes, err := Marshal(bp.ElectionVoteQc.Header)
		if err != nil {
			return nil, err
		}
		viewKey := BytesToString(viewBytes)
		if _, ok := deDupMap[viewKey]; !ok {
			deDupMap[viewKey] = make(map[string]struct{})
		}
		leader := bp.ElectionVoteQc.LeaderPublicKey
		if _, ok := deDupMap[viewKey][BytesToString(leader)]; ok {
			continue
		} else {
			// add to de-dup map
			m := deDupMap[viewKey]
			m[BytesToString(leader)] = struct{}{}
			deDupMap[viewKey] = m
			// add to infraction
			pubKeys = append(pubKeys, leader)
		}
	}
	return
}
