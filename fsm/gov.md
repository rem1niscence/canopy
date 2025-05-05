# gov.go - On-chain Governance System

This file implements the on-chain governance system for the Canopy blockchain, handling parameter changes, treasury subsidies, and proposal management. It provides the mechanisms for validators and stakeholders to propose, vote on, and implement changes to the blockchain's parameters.

## Overview

The governance system is designed to handle:
- Proposal validation and approval
- Parameter updates across different logical spaces
- State conformity to parameter changes
- Polling functionality for community feedback
- Protocol version management for feature activation
- Root chain identification and management

## Core Components

### Proposal Management

The proposal management system in the Canopy blockchain handles the validation and approval of governance proposals through a decentralized process. This system is a critical componen
t that enables on-chain governance, allowing the network to evolve and adapt through consensus.

Proposals in Canopy follow a specific lifecycle:

1. **Creation**: A community member creates a proposal in JSON format
2. **Distribution**: The proposal is shared with validators (typically via Discord or governance forums)
3. **Validator Configuration**: Validators decide whether to approve or reject the proposal
4. **Submission**: Once sufficient approval is gathered, the proposal can be submitted as a transaction

This allows validators to have control over which proposals they support, while still maintaining consensus through the two-thirds majority rule.

### Parameter Management and Spaces

Parameters in the Canopy blockchain are organized into logical "spaces" for
better organization, facilitating effective management and updates:

1. **Consensus Parameters**: Control fundamental blockchain behavior
   - Protocol version (stored as a string)
   - Maximum block size
   - Root chain ID

2. **Validator Parameters**: Govern validator behavior
   - Unstaking blocks
   - Maximum pause blocks
   - Slashing percentages for violations
   - Maximum committee size

3. **Fee Parameters**: Set transaction costs
   - Transaction fees
   - Operation fees

4. **Governance Parameters**: Control the governance process itself
   - Proposal requirements
   - Voting periods

Each parameter space contains specific parameters that can be updated through
governance proposals. Most parameters are primarily stored as uint64 values,
providing a clear structure for developers and users to understand the system.
This organization makes it easier to manage and update related parameters
together, ensuring that the critical components of the blockchain operate
smoothly.

### Polling System

The governance system includes a polling mechanism that allows for "straw polling" - non-binding votes that gauge community sentiment. This works by:
- Parsing transaction memos for poll commands
- Tracking votes from both validators and regular accounts
- Calculating results based on voting power (tokens)

This provides a way to gather feedback from the community without requiring formal on-chain governance proposals.

### Protocol Version Management

The protocol version system enables feature activation at specific heights. By updating the protocol version parameter, new features can be enabled once the blockchain reaches a certain height. This allows for coordinated upgrades without requiring hard forks.

### Parameter Update Mechanism

When a parameter is updated, the system follows these steps:

1. The previous parameters are saved for comparison
2. The parameter space is identified (Consensus, Validator, Fee, or Governance)
3. The parameter is updated with the new value
4. The updated parameter space is saved back to state
5. If necessary, the state is adjusted to conform to the new parameters

This process ensures that parameter updates are applied consistently and that the blockchain state remains valid after updates.

### Governance and Validator System

The governance system interacts closely with the validator system:

- When MaxCommitteeSize is reduced, the governance system must update validator committees
- Validator voting power determines the outcome of governance proposals
- Slashing parameters set by governance control validator penalties

This relationship ensures that validators operate within the rules set by governance while also giving them a say in how those rules evolve.

### Protocol Version and Feature Activation

The protocol version parameter interacts with feature activation:

- New features can check if they should be enabled based on the current protocol version
- Features can be programmed to activate at specific heights
- This allows for coordinated upgrades without requiring hard forks

For example, a new transaction type could be added to the code but remain inactive until the protocol version reaches a certain value and the blockchain reaches the specified height.

### Polling and Community Feedback

The polling system provides a way for the community to express opinions without formal governance:

- Transactions with special memo commands are parsed for poll votes
- Votes are weighted by token holdings (for accounts) or voting power (for validators)
- Results show approval percentages for both validators and regular accounts

This creates a feedback mechanism that can inform formal governance proposals and gauge community sentiment.

## Security & Integrity Mechansisms

- Height-based validation ensures proposals are only processed within their valid range
- Parameter validation prevents invalid values from being set
- State conformity ensures the blockchain remains in a valid state after parameter updates
- Protocol version management allows for controlled feature activation
- Root chain management maintains the security of the validator set
