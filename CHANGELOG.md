# CHANGELOG

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
