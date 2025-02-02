# CHANGELOG

## [Unreleased] - 1/29/2024
- Add more validator status to wallet

## [Unreleased] - 1/29/2024
- Display latest height in wallet

## [Unreleased] - 1/28/2024
- Remove net addr in wallet when not delegating

## [Unreleased] - 1/28/2024
- Add max amount form button

## [Unreleased] - 1/28/2024
- Fix UI transactions table Issue

## [Unreleased] - 1/24/2024
- Modified architecture to supported forkable multi-chain 
- Created 'buy-orders'
- Fixed polling and proposals

## [Unrelease] - 1/23/24
- Add Failure transactions cache
- Add Simple paginator query with no additional params
- Implement Failure transactions endpoint
- Display failed user transactions in the wallet
- Fix Wallet Bug on number input

## [Unreleased] - 1/21/2024
- Set up prettier for both Wallet and Explorer
>>>>>>> origin/main

## [Unreleased] - 1/20/2024
- Wallet UI fixes
- Added wallet text input sanitization
- Added wallet number input formatting 

## [Unreleased] - 1/19/2024
- Changed Canopy to act as a sub-chain to prepare for the sub-chain architecture
- Added various rpc calls to support this architecture

## [Unreleased] - 1/12/2024
- Removed json omitempty for Messages 
- Added `out` directories to git for Web Wallet & Block Explorer
- Auto-import the key-stores in the 2 node setup
- Added password prompt when first starting to import
- Added delegate and compound functionality to web wallet

## [Unreleased] - 12/30/2024
- Fixed total supply bug where it counts burned rewards
- Wallet UI fixes
- Added swap transactions to the web wallet
- Added polling to the web wallet

## [Unreleased] - 12/29/2024
- Added straw poll functionality (untested)

## [Unreleased] - 12/25/2024
- Fixed load certificate bug
- Fixed peer reputation issue with already in mempool
- Fixed peer reputation issue with block wrong height
- Fixed invalid block time issue when syncing
- Fixed rate limit issue

## [Unreleased] - 12/23/2024

- Fixed log level
- Removed HarmonyOne VDF implementation in favor of a memory optimized implementation (saves +1.5GB Mem)
- Fixed bft (process time) bug
- Fixed optimistic timer trigger
- Added logging to VDF
- 2x'd the round delay in the pacemaker
- Added volumes to create a 2 node network
- Deprecated bad_proposer logic

## [Unreleased] - 12/12/2024

- Added the SMT implementation in store package

## [Unreleased] - 12/5/2024

- Started the SMT logic in store package

## [Unreleased] - 12/4/2024

- Fixed bugs in the `block_explorer`
- Fixed `block-by-height` CLI bug
- Renamed `block-by-height` to block in cli
- Reverted `block timestamp` to unix micro

## [Unreleased] - 12/3/2024

- Fixed `RPC client` return types for the below endpoints
- Addressed the `committee stake` bug
- Retroactively filled in changelog for september and october gap
- Added replay attack protection logic for transactions

## [Unreleased] - 12/2/2024

- Added RPC and CLI functionality:
    - `Get Committee Members`
    - `Get Paid Committees`
    - `Get All Committees Data`
    - `Get Committee Data`
    - `Create Sell Order Transaction`
    - `Edit Sell Order Transaction`
    - `Delete Sell Order Transaction`
    - `Get Orders`
    - `Get Order`

## [Unreleased] - 12/1/2024

- Fixed startup bugs
- Removed redundant logging
- Added logic for gossiping block in single node network
- Added height update logic

## [Unreleased] - 11/18/2024

- Added unit tests to `lib` directory
- Updated Changelog

## [Unreleased] - 11/12/2024

- Improved vdf file comments

## [Unreleased] - 11/11/2024

- Finished commenting and enhancing tests in the crypto package

## [Unreleased] - 11/09/2024

- Finished commenting the .proto files
- Began commenting and enhancing tests in the crypto package

## [Unreleased] - 11/08/2024

- Commented to proto files bft.proto - tx.proto
- Removed param Block_Reward
- Added halvening logic

## [Unreleased] - 11/07/2024

- Finished comments to the .proto files
- Renamed the project from codename to `Canopy`
- Removed fuzzer (for now)

## [Unreleased] - 10/27/2024

- Added comments to `/lib/proto` codec
- Added comments to `/lib/proto` consensus
- Added comments to `/lib/proto` crypto
- Added comments to `/lib/proto` genesis
- Added comments to `/lib/proto` gov
- Added comments to `/lib/proto` message
- Added comments to `/lib/proto` p2p
- Added comments to `/lib/proto` peer
- Added comments to `/lib/proto` store
- Added comments to `/lib/proto` swap

## [Unreleased] - 10/13/2024

- Added comments to `/lib/proto` account
- Added comments to `/lib/proto` bft
- Added comments to `/lib/proto` certificate
- Added FSM tests for validator
- Commented that same file

## [Unreleased] - 09/29/2024

- Added FSM tests for swap
- Added FSM tests for transaction
- Added FSM tests for message
- Added FSM tests for state
- Commented the same files

## [Unreleased] - 09/15/2024

- Added FSM tests for byzantine
- Added FSM tests for committee
- Added FSM tests for genesis
- Added FSM tests for gov
- Commented the same files

## [Unreleased] - 09/01/2024

- Added FSM tests for account
- Added FSM tests for automatic
- Commented the same files

## [Unreleased] - 08/16/2024

- Fully documented the P2P module
- Update the peer book functionality
- Added saving routine for `book.json`

## [Unreleased] - 08/12/2024

- Added `Canopy` specific functionality
- Made `Canopy` just like every other chain
- Move to `plugin` model
- Added subsidy txn

## [Unreleased] - 08/10/2024

- Changed bft proposals from `Block` to `Proposal`
- `message_equity_grant` -> `message_proposal`

## [Unreleased] - 08/06/2024

- FSM changes for `Committees`
- FSM changes for `message_equity_grant`
- Proto changes to remove non-determinism i.e `maps`

## [Unreleased] - 07/30/2024

- Refactored `DSE` and `BPE` to `DoubleSignEvidences` and `BadProposerEvidences` respectively

## [Unreleased] - 07/22/2024

- Contained `DSE` and `BPE` to `bft package`
- Changed `BlockHeader` structure to be without the actual `DSE`

## [Unreleased] - 07/18/2024

- Simplified `evidence` collection

## [Unreleased] - 07/15/2024

- Added `bft` unit tests

## [Unreleased] - 06/30/2024

- Simplified `bft.vote` code

## [Unreleased] - 06/29/2024

- Simplified `bft.proposal` code

## [Unreleased] - 06/18/2024

- Refactored `producer/block` to `proposer/proposal` to be generic for other types of apps

## [Unreleased] - 06/17/2024

- Split `consensus` package into `bft` and `app`
- Modularized `bft` package

## [Unreleased] - 06/15/2024

- Added `block explorer` hosting to rpc
- Added `web wallet` hosting to rpc

## [Unreleased] - 06/03/2024

- Added `web wallet` first implementation
- Added `rpc server functionallity`
- Added `rpc client functionality`
- Added `cli functionality`
- Added `msg pause`
- Added all tx types to `fuzzer`
- Fixed various bugs

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
