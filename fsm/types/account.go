package types

import (
	"bytes"
	"github.com/ginchuco/ginchu/lib"
	"sort"
)

func (x *Supply) Sort() {
	x.SortCommittees()
	x.SortDelegations()
}

func (x *Supply) SortCommittees() {
	sort.Slice(x.CommitteesWithDelegations, func(i, j int) bool {
		return x.CommitteesWithDelegations[i].Amount >= x.CommitteesWithDelegations[j].Amount
	})
}

func (x *Supply) SortDelegations() {
	sort.Slice(x.DelegationsOnly, func(i, j int) bool {
		return x.DelegationsOnly[i].Amount >= x.DelegationsOnly[j].Amount
	})
}

func (x *Equity) Combine(e *Equity) lib.ErrorI {
	if e == nil {
		return nil
	}
	for _, ep := range e.EquityPoints {
		x.addPoints(ep.Address, ep.Points)
	}
	return nil
}

func (x *Equity) AwardPoints(equityPoints []*EquityPoints) lib.ErrorI {
	x.NumberOfSamples++
	for _, ep := range equityPoints {
		x.addPoints(ep.Address, ep.Points)
	}
	return nil
}

func (x *Equity) addPoints(address []byte, basisPoints uint64) {
	for i, ep := range x.EquityPoints {
		if bytes.Equal(address, ep.Address) {
			x.EquityPoints[i].Points += ep.Points
			return
		}
	}
	x.EquityPoints = append(x.EquityPoints, &EquityPoints{
		Address: address,
		Points:  basisPoints,
	})
}
