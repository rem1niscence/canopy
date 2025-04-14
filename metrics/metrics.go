package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// NodeStatus represents the current status of the node
	NodeStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canopy_node_status",
		Help: "Current status of the node (1 for active, 0 for inactive)",
	})

	// NodeSyncingStatus represents whether the node is syncing
	NodeSyncingStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canopy_node_syncing_status",
		Help: "Node syncing status (1 for syncing, 0 for synced)",
	})

	// PeerMetrics represents various peer-related metrics
	PeerMetrics = struct {
		TotalPeers    *prometheus.GaugeVec
		InboundPeers  *prometheus.GaugeVec
		OutboundPeers *prometheus.GaugeVec
	}{
		TotalPeers: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_peer_total",
			Help: "Total number of peers",
		}, []string{"status"}),
		InboundPeers: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_peer_inbound",
			Help: "Number of inbound peers",
		}, []string{"status"}),
		OutboundPeers: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_peer_outbound",
			Help: "Number of outbound peers",
		}, []string{"status"}),
	}

	// BFTMetrics represents BFT-related metrics
	BFTMetrics = struct {
		Height     prometheus.Gauge
		RootHeight prometheus.Gauge
	}{
		Height: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "canopy_bft_height",
			Help: "Current BFT height",
		}),
		RootHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "canopy_bft_root_height",
			Help: "Current BFT root height",
		}),
	}

	// GovernanceMetrics represents governance-related metrics
	GovernanceMetrics = struct {
		ActiveProposals prometheus.Gauge
	}{
		ActiveProposals: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "canopy_governance_active_proposals",
			Help: "Number of active governance proposals",
		}),
	}

	// ValidatorMetrics represents validator-related metrics
	ValidatorMetrics = struct {
		Status      *prometheus.GaugeVec
		Type        *prometheus.GaugeVec
		Compounding *prometheus.GaugeVec
		Balance     *prometheus.GaugeVec
	}{
		Status: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_validator_status",
			Help: "Validator status (0: Unstaked, 1: Staked, 2: Unstaking, 3: Paused)",
		}, []string{"address"}),
		Type: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_validator_type",
			Help: "Validator type (0: Delegate, 1: Validator)",
		}, []string{"address"}),
		Compounding: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_validator_compounding",
			Help: "Validator compounding status (1: true, 0: false)",
		}, []string{"address"}),
		Balance: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "canopy_account_balance",
			Help: "Account balance in base units",
		}, []string{"address"}),
	}

	// TransactionMetrics represents transaction-related metrics
	TransactionMetrics = struct {
		Received *prometheus.CounterVec
		Sent     *prometheus.CounterVec
	}{
		Received: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "canopy_transaction_received",
			Help: "Number of received transactions",
		}, []string{"address"}),
		Sent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "canopy_transaction_sent",
			Help: "Number of sent transactions",
		}, []string{"address"}),
	}

	// TransactionCount represents the total number of transactions processed
	TransactionCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "canopy_transaction_count",
		Help: "Total number of transactions processed",
	})

	// BlockProcessingTime represents the time taken to process blocks
	BlockProcessingTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "canopy_block_processing_seconds",
		Help:    "Time taken to process blocks in seconds",
		Buckets: prometheus.DefBuckets,
	})

	// MemoryUsage represents the current memory usage in bytes
	MemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canopy_memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})

	BlocksValidatedByProposer = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canopy_blocks_validated_by_proposer",
		Help: "Total number of blocks validated when node is proposer",
	})
)

func init() {
	prometheus.MustRegister(BlocksValidatedByProposer)
}
