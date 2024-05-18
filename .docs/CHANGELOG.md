# CHANGELOG

## [Unreleased] - 05/18/2024

- Added `block explorer` first implementation
- Fixed `mempool delete` issue

## [Unreleased] - 05/02/2024

- `RPC` module
- Testing transaction `fuzzer`

## [Unreleased] - 04/28/2024

- Startup functionality
- Single node `blockchain` functionality
- Basic logging in `consensus` module
- Fixed `genesis` bugs

## [Unreleased] - 04/25/2024

- Changed `evidence` to be accepted from any height
- Index `double_signers`
- Added genesis and startup functionality
- Data directory functionality
- Genesis block is `1` not `0`

## [Unreleased] - 04/21/2024

- Cleanup `consensus` module
- Fixed `syncing` multithreading issues
- Changed `Governance` to be Validator vote

## [Unreleased] - 04/19/2024

- Massive project restructuring and overhaul
- Cleanup in `consensus/vote.go` module
- Update thread safety in `consensus`

## [Unreleased] - 04/12/2024

- Added `encrypted` p2p connection
- Added `multiplexed` p2p connection
- Added `peerBook` functionality
- Added `gossip` functionality
- Added `peerList` functionality

## [Unreleased] - 04/11/2024

- Added `byzantine` evidence logic to consensus
- Added `lastProducers` logic to state_machine
- Reorganize, linter, and license

## [Unreleased] - 04/09/2024

- Added `syncing` logic
- Started `byzantine` evidence logic
- Added indexing for `evidence` and `qc`
- Added `p2p` TODO outline
- Updated `block` structure with `last_qc` and `evidence`

## [Unreleased] - 04/06/2024

- Consolidated `leader election` and `consensus` packages
- Added verification to `Message` and `AggrSig` structures
- Added `consensus` diagrams
- Cleanup up `Consensus State` lifecycle
- Fixed `hvs` and `hlm` structures

## [Unreleased] - 03/30/2024

- Added `leader_election` VRF and CDF
- Continued `hotstuff` logic
- Added `consensus` messages
- Added `consensus state` lifecycle
- Added `bls` helper functions
- Added `p2p` interface
- Added `app` interface
- Added `vote` and `leader` message container structures
- Added `consensus` documentation

## [Unreleased] - 03/29/2024

- Added `bls` crypto
- Started `hotstuff` logic
- Added `consensus.proto` types
- Added outline for `practical vrf` for leader selection

## [Unreleased] - 03/25/2024

- Added `validators_root` to block header
- Added `next_validators_root` to block header
- Added `merkle root` functionality
- Added `app/app.go` functionality

## [Unreleased] - 03/24/2024

- Converted `store` `errors` to `ErrorI`
- Fixed `valid->invalid` store bug
- Added `Mempool` functionality to app
- Added `Block` functionality to app
- Added `State` functionality to app
- Added `Block` functionality to indexer

## [Unreleased] - 03/21/2024

- Added `txn` to store package
- Added `NewTxn()` to `StoreI`
- Began `Mempool` and `App` logic

## [Unreleased] - 03/16/2024

- Added `CheckTransaction` functionality
- Added `ApplyTransaction` functionality

## [Unreleased] - 03/15/2024

- Added `TransactionIndexer` to store package
- Added interfaces: `TransactionResultI`, `TransactionI`, `SignatureI`, `TransactionIndexerI`

## [Unreleased] - 03/12/2024

- Added `consensus_validator` keys logic and functionality
- Added `end_block` logic
- Added `genesis` logic
- Added `params.validate()` logic

## [Unreleased] - 03/11/2024

- Added `begin_block` logic
- Added funcs for `auto-unstaking` and `auto-max-paused`
- Updated notes with `Canopy` details
- Added `state machine` handling of `byzantine evidence` in notes

## [Unreleased] - 03/10/2024

- Added handler for `change_param`
- Added handler for `double_sign`
- Added `automatic.go` file

## [Unreleased] - 03/08/2024

- Created `StateMachine` structure
- Added `handler` functionality to `state_machine`
- Added `check` functionality to `messages`
- Added `account` functionality to `state_machine`
- Added `pool` functionality to `state_machine`
- Added `param` functionality to `state_machine`
- Simplified `store` interfaces and utilized them in the `store` package
- Updated proto package to reflect the proper directories
- Added `account` to proto package
- Added `pool` to proto package

## [Unreleased] - 02/28/2024

- Added `SMT` tests to `store` package
- Changed store design to utilize `badgerdb` and `badger.txn`

## [Unreleased] - 02/25/2024

- Added `store` tests
- Added `Store` implementation for `StoreI`

## [Unreleased] - 02/24/2024

- Added `transaction` type
- Added `send`, `stake`, and `unstake` message types
- Added `change parameter` message type
- Added `genesis` state type
- Added `params` type
- Added `account` type
- Added `validator` type
- Added `leveldb` for state store
- Added `celestia` SMT for state commitment

## [Unreleased] - 02/23/2024

- Added `CHANGELOG.md`
- Added `ed25519` functionality
- Added `hash` functionality
- Added `codec` functionality
- Added `protobuf` generate file
- Added `log` functionality
- Added `docker`
- Added `github` workflow
