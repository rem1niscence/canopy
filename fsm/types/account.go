package types

import (
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
