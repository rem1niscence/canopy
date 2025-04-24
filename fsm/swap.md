# swap.go

The `swap.go` file contains essential logic related to the state machine changes within the Canopy Network, specifically focusing on the processes involved in token swapping. This document details how the state machine handles various operations associated with orders on the blockchain, particularly those involving buy and sell transactions.

## Overview

The `swap.go` file coordinates the following key operations:
- Handling committee swaps for sell orders
- Managing lock orders and reset orders for transactions
- Closing orders that are successfully completed
- Processing the root chain order book and ensuring the correct association between transactions and orders

## Core Components

### StateMachine

The core component of the `swap.go` file is the `StateMachine`, which governs the lifecycle of orders based on the state of transactions in the blockchain. It maintains consistency by observing the committee's decisions and updating the order book accordingly.

### Order Management

This component oversees the creation, modification, locking, resetting, and closing of orders:
- **Lock Orders**: Represents orders that are temporarily reserved by a buyer, allowing them time to fulfill the transaction before the deadline.
- **Reset Orders**: Orders that need to be re-listed if the buyer does not perform the necessary actions before the deadline, ensuring the marketplace remains active.
- **Close Orders**: Orders that are finalized when the buyer satisfactorily sends the required tokens, allowing the seller to receive their payment.

### Transaction Processing

The file handles transactions specifically labeled as "send" types, adhering to the protocol defined by the network. It scrutinizes each transaction to determine if it affects existing orders, ensuring that the system accurately reflects the current state of the order book.

### Order Book Management

The order book component enables the system to maintain a clear record of all active sell orders. It includes functionalities to:
- Track the amount sent for each order
- Identify whether orders are still locked or have expired
- Confirm whether an order should be classified as closed, based on received transactions matching the expected amounts

## Technical Details

### Committee Swaps

The swapping mechanism is triggered when the committee submits a certificate results transaction, indicating different statuses for orders:
- **Buy**: Claiming or reserving the rights to a sell order.
- **Reset**: Re-opening orders that buyers did not complete before their deadlines.
- **Close**: Finalizing orders where buyers successfully sent tokens to sellers within the stipulated time frames.

### Parsing Transaction Data

The system employs functions to extract and validate embedded orders within transactions. By parsing transaction memos, it identifies critical information necessary for processing lock and close orders, ensuring that the appropriate actions are executed.

### Handling Expired and Incomplete Orders

In order to efficiently manage the order lifecycle, the logic includes checks to identify and reset expired orders while ensuring that completed transactions are recognized. This keeps the order book responsive to real-time changes within the network's state.

## Component Interactions

### Processing Transactions

Transactions are processed by first filtering relevant "send" transactions and then verifying them against existing orders. The outcomes of these verifications drive the updates to both the transferred amounts and the overall state of the associated orders.

### Cross-Referencing with Blockchain

The interactions with the blockchain occur through reading block results and transaction data, which include:
- Confirmation of transactions and their effects on the order book.
- Historical checks against previous blocks to ensure the integrity of transactions that may result from asynchronous actions.

### Error Handling

In scenarios where transactions do not conform to expectations (e.g., malformed messages or failed order retrievals), appropriate logging and error management strategies are employed to maintain system reliability.

## Conclusion

The `swap.go` file plays a critical role in the functionality of the Canopy Network by managing token swaps, maintaining order integrity, and facilitating a seamless transaction process. The designed structure ensures that all market actions are efficiently handled, supporting a robust and responsive trading environment within the blockchain.
