# prop.go - Proposal Management in Byzantine Fault Tolerance Consensus

The prop.go file implements the proposal management system for the Byzantine Fault Tolerance (BFT) consensus mechanism in the Canopy blockchain. It handles how consensus proposals from leader nodes are stored, retrieved, and validated during the consensus process.

## Overview

This file is designed to handle:
- Storage of proposals received from leader nodes
- Validation of proposals based on cryptographic proofs
- Management of election candidates during leader selection
- Retrieval of proposals for consensus processing
- Handling committee resets from the root chain

## Core Components

### ProposalsForHeight

A data structure that maintains an in-memory collection of messages received from leader nodes (proposers). It organizes proposals by round and phase, allowing the system to track multiple proposals during the election phase when there can be multiple candidates competing to become the next leader.

The structure is particularly important because in BFT consensus, nodes need to keep track of proposals to verify that the leader is following the protocol correctly and to participate in voting.

### Proposal Management

The system provides functions to add, retrieve, and reset proposals:
- Proposals are added to the appropriate round and phase
- Election proposals are handled differently, allowing multiple candidates
- Non-election proposals overwrite previous ones since there should only be one valid proposal per phase
- Proposals can be reset when a new committee is formed

### Election Candidate Verification

During the election phase, the system verifies candidates using a Verifiable Random Function (VRF). This cryptographic mechanism ensures that leader selection is both random and verifiable, preventing manipulation of the election process.

The verification process checks:
- The validator's identity and voting power
- The cryptographic proof (VRF signature)
- Whether the validator qualifies as a candidate based on sortition rules

## Security Mechanisms

The proposal system incorporates several security features to maintain consensus integrity. Proposals must be justified by either a VRF proof (for elections) or a Quorum Certificate (QC) containing votes from at least two-thirds of validators for the previous phase. The system also prevents duplicate election candidates and verifies cryptographic signatures to ensure authenticity. During committee resets, the system preserves election candidates to prevent message loss while establishing a clean state for new consensus rounds.
