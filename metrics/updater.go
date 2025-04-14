package metrics

import (
	"time"
)

// UpdateNodeSyncingStatus updates the node syncing status
func UpdateNodeSyncingStatus(isSyncing bool) {
	if isSyncing {
		NodeSyncingStatus.Set(1)
	} else {
		NodeSyncingStatus.Set(0)
	}
}

// UpdatePeerMetrics updates all peer-related metrics
func UpdatePeerMetrics(total, inbound, outbound int) {
	PeerMetrics.TotalPeers.WithLabelValues("connected").Set(float64(total))
	PeerMetrics.InboundPeers.WithLabelValues("connected").Set(float64(inbound))
	PeerMetrics.OutboundPeers.WithLabelValues("connected").Set(float64(outbound))
}

// UpdateBFTMetrics updates BFT-related metrics
func UpdateBFTMetrics(height, rootHeight uint64) {
	BFTMetrics.Height.Set(float64(height))
	BFTMetrics.RootHeight.Set(float64(rootHeight))
}

// UpdateGovernanceMetrics updates governance-related metrics
func UpdateGovernanceMetrics(activeProposals int) {
	GovernanceMetrics.ActiveProposals.Set(float64(activeProposals))
}

// UpdateValidatorStatus updates the status of a validator
func UpdateValidatorStatus(address string, status int) {
	ValidatorMetrics.Status.WithLabelValues(address).Set(float64(status))
}

// UpdateValidatorType updates the type of a validator
func UpdateValidatorType(address string, validatorType int) {
	ValidatorMetrics.Type.WithLabelValues(address).Set(float64(validatorType))
}

// UpdateValidatorCompounding updates the compounding status of a validator
func UpdateValidatorCompounding(address string, isCompounding bool) {
	if isCompounding {
		ValidatorMetrics.Compounding.WithLabelValues(address).Set(1)
	} else {
		ValidatorMetrics.Compounding.WithLabelValues(address).Set(0)
	}
}

// UpdateAccountBalance updates the balance of an account
func UpdateAccountBalance(address string, balance uint64) {
	ValidatorMetrics.Balance.WithLabelValues(address).Set(float64(balance))
}

// UpdateTransactionMetrics updates transaction-related metrics for an address
func UpdateTransactionMetrics(address string, received, sent uint64) {
	TransactionMetrics.Received.WithLabelValues(address).Add(float64(received))
	TransactionMetrics.Sent.WithLabelValues(address).Add(float64(sent))
}

// UpdateBlockProcessingTime updates the block processing time metric
func UpdateBlockProcessingTime(duration time.Duration) {
	BlockProcessingTime.Observe(duration.Seconds())
}

// UpdateMemoryUsage updates the memory usage metric
func UpdateMemoryUsage(bytes uint64) {
	MemoryUsage.Set(float64(bytes))
}

// UpdateBlocksValidatedByProposer increments the counter for blocks validated when node is proposer
func UpdateBlocksValidatedByProposer() {
	BlocksValidatedByProposer.Inc()
}

// Constants for validator status
const (
	ValidatorStatusUnstaked  = 0
	ValidatorStatusStaked    = 1
	ValidatorStatusUnstaking = 2
	ValidatorStatusPaused    = 3
)

// Constants for validator type
const (
	ValidatorTypeDelegate  = 0
	ValidatorTypeValidator = 1
)
