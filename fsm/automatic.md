# automatic.go - Automatic State Changes in Blockchain Blocks

This file handles automatic state changes that occur at the beginning and end of a block in the Canopy blockchain. Unlike transaction-induced changes, these are system-level operations that happen automatically as part of the block processing lifecycle.

## Overview

The automatic.go file implements critical functions that:
- Execute state changes at block boundaries
- Enforce protocol upgrades
- Manage validator rewards and punishments
- Handle certificate results from consensus
- Maintain validator participation records

## Core Components

### Block Lifecycle Handlers

The file contains two primary functions that bookend block processing: BeginBlock and EndBlock. These functions handle state changes that must occur automatically at specific points in the block lifecycle, rather than being triggered by user transactions.

BeginBlock runs before any transactions are processed and handles protocol version enforcement, committee reward pool funding, and certificate result processing. EndBlock executes after all transactions have been applied and manages proposer tracking, reward distribution, and validator unstaking.

This separation ensures that certain critical blockchain operations happen in a predictable order regardless of what transactions are included in a block.

### Protocol Version Management

The blockchain enforces protocol upgrades through version checking. When a new protocol version is activated through governance parameters, all nodes must upgrade their software to remain compatible with the network.

The system checks if the current height has reached the activation height for a new protocol version. If so, nodes running older software versions will be unable to process blocks, forcing them to upgrade to maintain consensus with the network.

### Certificate Result Processing

Certificate results represent the outcome of Byzantine Fault Tolerance (BFT) consensus rounds. The system handles these results differently depending on whether the current chain is the root chain or a nested chain.

For the root chain, certificate results are processed automatically during BeginBlock. For nested chains, certificate results are handled through explicit transactions. This design allows the root chain to coordinate the security of multiple nested chains while maintaining separation between them.

### Validator Management

The system automatically manages validator participation through several mechanisms:

1. Tracking proposers who created recent blocks to ensure fair leader election
2. Distributing rewards to committees based on their participation
3. Force-unstaking validators who have been paused for too long
4. Removing validators who have completed the unstaking process

These automatic processes ensure that validators who aren't actively participating are eventually removed, maintaining the health and security of the network.

### Checkpoint Handling

The system provides "checkpoint-as-a-service" functionality, allowing both the root chain and nested chains to record checkpoints. These checkpoints serve as verifiable records of blockchain state at specific heights, which can be used for various security and recovery purposes.

The checkpoint system ensures that only newer checkpoints can be recorded, preventing potential attacks that might try to rewrite history with older checkpoints.

### Committee Retirement

Committees (groups of validators securing a specific chain) can be retired when they're no longer needed. This can happen automatically when a nested chain signals that it wants to retire its committee, or through governance decisions.

When a committee is retired, it stops receiving subsidies and rewards, effectively ending its role in securing that particular chain.

### Forced Unstaking

Validators who pause their participation for too long (exceeding MaxPauseBlocks) are automatically unstaked by the system. This prevents validators from indefinitely pausing to avoid responsibilities while still holding their stake.

The system maintains a record of paused validators and their pause duration. When the maximum pause duration is reached, the validator is force-unstaked and their pause record is deleted.

### Last Proposer Tracking

The system maintains a record of the last five block proposers. This information is used in the leader election process to ensure fair distribution of block production opportunities and to prevent any single validator from dominating block production.

The proposer addresses are stored in a circular buffer, with the position determined by the current block height modulo 5. This creates a rolling window of recent proposers that can be used in the leader election algorithm.
