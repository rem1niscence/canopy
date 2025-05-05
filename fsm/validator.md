# validator.go - Validator Management in the Canopy Blockchain

This file implements state actions for Validators and Delegators in the Canopy blockchain. It provides functionality to manage validators, which are essential participants in the blockchain's consensus mechanism.

## Overview

The validator.go file handles:
- Retrieving and storing validator information
- Managing validator stake amounts
- Handling validator status changes (pausing, unpausing, unstaking)
- Filtering validators based on various criteria
- Managing validator committees

## Core Components

### Validators

Validators are participants in the Canopy blockchain who help secure the network by staking tokens and participating in consensus. Each validator has:
- An address that uniquely identifies them
- A public key used for cryptographic operations
- A stake amount representing their financial commitment to the network
- Committee assignments indicating which blockchain segments they validate
- Status information (active, paused, or unstaking)

Validators can be regular validators or delegators (validators who delegate their stake to committees).

### Validator State Management

The file provides comprehensive functionality for managing validator state within the blockchain:
- Creating and retrieving validators
- Updating validator information
- Tracking validator stake amounts
- Managing validator status (active, paused, unstaking)
- Handling validator committees

When validators change their stake or status, the system updates not only the validator records but also related state information like supply tracking and committee assignments.

### Validator Lifecycle

Validators go through different states during their lifecycle:
1. **Active**: Participating in consensus and earning rewards
2. **Paused**: Temporarily inactive (can be automatic or manual)
3. **Unstaking**: In the process of withdrawing their stake
4. **Deleted**: Completely removed from the validator set

The system tracks these state transitions and ensures proper handling of validator funds and responsibilities throughout the lifecycle.

### Validator Staking Process

The staking process involves several steps:
1. A validator commits tokens to the network (their "stake")
2. The validator is assigned to committees based on their stake amount
3. The staked tokens are locked and cannot be used for other purposes
4. The validator participates in consensus and earns rewards

The stake amount is crucial as it:
- Determines the validator's influence in the network
- Serves as collateral that can be slashed for misbehavior
- Affects which committees the validator is assigned to

### Validator Unstaking Process

When a validator wants to withdraw their stake:
1. They initiate unstaking by setting an "unstaking height"
2. The validator is removed from their committees
3. Their tokens remain locked until the unstaking period completes
4. At the specified block height, their tokens are returned to their output address

This process ensures validators can't immediately withdraw after misbehaving and provides stability to the network.

### Validator Pausing Mechanism

Validators can be paused either manually or automatically:
1. Manual pausing occurs when a validator chooses to temporarily stop participating
2. Automatic pausing happens when a validator fails to participate properly
3. Paused validators have a "max paused height" after which they are force-unstaked
4. Validators can unpause before reaching this height to resume normal operation

This mechanism helps maintain network health by removing non-participating validators while giving them a chance to resolve issues.

### Validator and Supply Tracking

When validators stake or unstake tokens, the system must update various supply trackers:
- Total supply: The total amount of tokens in the system
- Staked supply: Tokens that are currently staked by validators
- Delegated supply: Tokens that are staked by delegators

These supply trackers help maintain an accurate picture of token distribution and ensure the economic model of the blockchain functions correctly.

### Validators and Committees

Validators are assigned to committees based on their stake amount:
1. When a validator stakes tokens, they're assigned to committees
2. If a validator increases their stake, they may be assigned to additional committees
3. When a validator unstakes or is slashed, they're removed from committees
4. Committee assignments affect which parts of the blockchain the validator helps secure

This committee system allows the blockchain to distribute validation work efficiently across the network.

### Authorization and Security

The system implements security measures for validator operations:
- Only authorized signers can perform actions on behalf of a validator
- For non-custodial validators, both the validator address and output address can sign
- For custodial validators, only the validator address can sign
- Public key verification ensures only legitimate validators can participate

These security measures protect validators and the network from unauthorized actions.

## Security & Integry Mechansisms

- **Slashing**: Validators who misbehave can have a portion of their stake taken as punishment
- **Pausing**: Validators who fail to participate are automatically paused
- **Force-unstaking**: Validators who remain paused too long are force-unstaked
- **Authorization checks**: Only authorized addresses can perform actions for a validator
- **Filtering**: Validators can be filtered based on various criteria for monitoring purposes
