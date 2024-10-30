package types

import (
	"sort"
)

// Sort() sorts the list of Pools for committees and delegations by amount (stake) high to low
func (x *Supply) Sort() {
	x.SortCommittees()
	x.SortDelegations()
}

// SortCommittees() sorts the committees list by amount (stake) high to low
func (x *Supply) SortCommittees() {
	FilterAndSortPool(&x.CommitteesWithDelegations)
}

// SortCommittees() sorts the delegations list by amount (stake) high to low
func (x *Supply) SortDelegations() {
	FilterAndSortPool(&x.DelegationsOnly)
}

// filterAndSort() removes zero and nil elements from the pool slice and then sorts the slice by amount
// finally setting the result to the pointer from the parameter
func FilterAndSortPool(x *[]*Pool) {
	if x == nil {
		return
	}
	// filter zero elements
	result := make([]*Pool, 0, len(*x))
	for _, v := range *x {
		if v != nil && v.Amount != 0 {
			result = append(result, v)
		}
	}
	// sort the slice by amount
	sort.Slice(result, func(i, j int) bool {
		// second order sort is ascending by Ids
		if result[i].Amount == result[j].Amount {
			return result[i].Id < result[j].Id
		}
		// descending by amount
		return result[i].Amount > result[j].Amount
	})
	// set to the pointer
	*x = result
}
