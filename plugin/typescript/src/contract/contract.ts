/* This file contains the base contract implementation that overrides the basic 'transfer' functionality */

import Long from 'long';

import { types } from '../proto/types.js';

import {
    IPluginError,
    ErrInsufficientFunds,
    ErrInvalidAddress,
    ErrInvalidAmount,
    ErrInvalidMessageCast,
    ErrTxFeeBelowStateLimit,
} from './error.js';

import type { Plugin, Config } from './plugin.js';
import { JoinLenPrefix, FromAny, Unmarshal } from './plugin.js';
import { fileDescriptorProtos } from '../proto/descriptors.js';

// ContractConfig: the configuration of the contract
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const ContractConfig: any = {
    name: "go_plugin_contract",
    id: 1,
    version: 1,
    supportedTransactions: ["send", "reward", "faucet"],
    transactionTypeUrls: [
        "type.googleapis.com/types.MessageSend",
        "type.googleapis.com/types.MessageReward",
        "type.googleapis.com/types.MessageFaucet",
    ],
    eventTypeUrls: [],
    fileDescriptorProtos,
};

// Contract() defines the smart contract that implements the extended logic of the nested chain
export class Contract {
    Config: Config;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    FSMConfig: any;
    plugin: Plugin;
    fsmId: Long;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    constructor(config: Config, fsmConfig: any, plugin: Plugin, fsmId: Long) {
        this.Config = config;
        this.FSMConfig = fsmConfig;
        this.plugin = plugin;
        this.fsmId = fsmId;
    }

    // Genesis() implements logic to import a json file to create the state at height 0 and export the state at any height
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    Genesis(_request: any): any {
        return {}; // TODO map out original token holders
    }

    // BeginBlock() is code that is executed at the start of `applying` the block
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    BeginBlock(_request: any): any {
        return {};
    }

    // EndBlock() is code that is executed at the end of 'applying' a block
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    EndBlock(_request: any): any {
        return {};
    }

    // CheckMessageSend() statelessly validates a 'send' message
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    CheckMessageSend(msg: any): any {
        // check sender address
        if (!msg.fromAddress || msg.fromAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check recipient address
        if (!msg.toAddress || msg.toAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check amount
        const amount = msg.amount as Long | number | undefined;
        if (!amount || (Long.isLong(amount) ? amount.isZero() : amount === 0)) {
            return { error: ErrInvalidAmount() };
        }
        // return the authorized signers
        return {
            recipient: msg.toAddress,
            authorizedSigners: [msg.fromAddress],
        };
    }

    // CheckMessageReward() statelessly validates a 'reward' message
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    CheckMessageReward(msg: any): any {
        // check admin address (must be 20 bytes)
        if (!msg.adminAddress || msg.adminAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check recipient address
        if (!msg.recipientAddress || msg.recipientAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check amount
        const amount = msg.amount as Long | number | undefined;
        if (!amount || (Long.isLong(amount) ? amount.isZero() : amount === 0)) {
            return { error: ErrInvalidAmount() };
        }
        // return the authorized signers (admin must sign this tx)
        return {
            recipient: msg.recipientAddress,
            authorizedSigners: [msg.adminAddress],
        };
    }

    // CheckMessageFaucet() statelessly validates a 'faucet' message
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    CheckMessageFaucet(msg: any): any {
        // check signer address (must be 20 bytes)
        if (!msg.signerAddress || msg.signerAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check recipient address
        if (!msg.recipientAddress || msg.recipientAddress.length !== 20) {
            return { error: ErrInvalidAddress() };
        }
        // check amount
        const amount = msg.amount as Long | number | undefined;
        if (!amount || (Long.isLong(amount) ? amount.isZero() : amount === 0)) {
            return { error: ErrInvalidAmount() };
        }
        // return the authorized signers (signer must sign this tx)
        return {
            recipient: msg.recipientAddress,
            authorizedSigners: [msg.signerAddress],
        };
    }
}

// Async versions of contract methods for proper state handling
export class ContractAsync {
    // CheckTx() is code that is executed to statelessly validate a transaction
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    static async CheckTx(contract: Contract, request: any): Promise<any> {
        // validate fee
        const [resp, err] = await contract.plugin.StateRead(contract, {
            keys: [
                { queryId: Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER)), key: KeyForFeeParams() },
            ],
        });

        if (err) {
            return { error: err };
        }
        if (resp?.error) {
            return { error: resp.error };
        }

        // convert bytes into fee parameters
        const feeParamsBytes = resp?.results?.[0]?.entries?.[0]?.value;
        if (feeParamsBytes && feeParamsBytes.length > 0) {
            const [minFees, unmarshalErr] = Unmarshal(feeParamsBytes, types.FeeParams);
            if (unmarshalErr) {
                return { error: unmarshalErr };
            }
            // check for the minimum fee
            const txFee = request.tx?.fee as Long | number | undefined;
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const sendFee = (minFees as any)?.sendFee as Long | number | undefined;
            if (txFee !== undefined && sendFee !== undefined) {
                const txFeeNum = Long.isLong(txFee) ? txFee.toNumber() : txFee;
                const sendFeeNum = Long.isLong(sendFee) ? sendFee.toNumber() : sendFee;
                if (txFeeNum < sendFeeNum) {
                    return { error: ErrTxFeeBelowStateLimit() };
                }
            }
        }

        // get the message and its type
        const [msg, msgType, msgErr] = FromAny(request.tx?.msg);
        if (msgErr) {
            return { error: msgErr };
        }
        // handle the message based on type
        if (msg) {
            switch (msgType) {
                case 'MessageSend':
                    return contract.CheckMessageSend(msg);
                case 'MessageReward':
                    return contract.CheckMessageReward(msg);
                case 'MessageFaucet':
                    return contract.CheckMessageFaucet(msg);
                default:
                    return { error: ErrInvalidMessageCast() };
            }
        }
        return { error: ErrInvalidMessageCast() };
    }

    // DeliverTx() is code that is executed to apply a transaction
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    static async DeliverTx(contract: Contract, request: any): Promise<any> {
        // get the message and its type
        const [msg, msgType, err] = FromAny(request.tx?.msg);
        if (err) {
            return { error: err };
        }
        // handle the message based on type
        if (msg) {
            switch (msgType) {
                case 'MessageSend':
                    return ContractAsync.DeliverMessageSend(contract, msg, request.tx?.fee as Long);
                case 'MessageReward':
                    return ContractAsync.DeliverMessageReward(contract, msg, request.tx?.fee as Long);
                case 'MessageFaucet':
                    return ContractAsync.DeliverMessageFaucet(contract, msg);
                default:
                    return { error: ErrInvalidMessageCast() };
            }
        }
        return { error: ErrInvalidMessageCast() };
    }

    // DeliverMessageSend() handles a 'send' message
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    static async DeliverMessageSend(contract: Contract, msg: any, fee: Long | number | undefined): Promise<any> {
        const fromQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));
        const toQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));
        const feeQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));

        // calculate the from key and to key
        const fromKey = KeyForAccount(msg.fromAddress!);
        const toKey = KeyForAccount(msg.toAddress!);
        const feePoolKey = KeyForFeePool(Long.fromNumber(contract.Config.ChainId));

        // get the from and to account
        const [response, readErr] = await contract.plugin.StateRead(contract, {
            keys: [
                { queryId: feeQueryId, key: feePoolKey },
                { queryId: fromQueryId, key: fromKey },
                { queryId: toQueryId, key: toKey },
            ],
        });

        // check for internal error
        if (readErr) {
            return { error: readErr };
        }
        // ensure no error fsm error
        if (response?.error) {
            return { error: response.error };
        }

        // get the from bytes and to bytes
        let fromBytes: Uint8Array | null = null;
        let toBytes: Uint8Array | null = null;
        let feePoolBytes: Uint8Array | null = null;

        for (const resp of response?.results || []) {
            const qid = resp.queryId as Long;
            if (qid.equals(fromQueryId)) {
                fromBytes = resp.entries?.[0]?.value || null;
            } else if (qid.equals(toQueryId)) {
                toBytes = resp.entries?.[0]?.value || null;
            } else if (qid.equals(feeQueryId)) {
                feePoolBytes = resp.entries?.[0]?.value || null;
            }
        }

        // convert the bytes to account structures
        const [fromRaw, fromErr] = Unmarshal(fromBytes || new Uint8Array(), types.Account);
        if (fromErr) {
            return { error: fromErr };
        }
        const [toRaw, toErr] = Unmarshal(toBytes || new Uint8Array(), types.Account);
        if (toErr) {
            return { error: toErr };
        }
        const [feePoolRaw, feePoolErr] = Unmarshal(feePoolBytes || new Uint8Array(), types.Pool);
        if (feePoolErr) {
            return { error: feePoolErr };
        }

        // Cast to any for property access
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const from = fromRaw as any;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const to = toRaw as any;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const feePool = feePoolRaw as any;

        // add fee to 'amount to deduct'
        const msgAmount = Long.isLong(msg.amount) ? msg.amount : Long.fromNumber(msg.amount as number || 0);
        const feeAmount = Long.isLong(fee) ? fee : Long.fromNumber(fee as number || 0);
        const amountToDeduct = msgAmount.add(feeAmount);

        // get from amount
        const fromAmount = Long.isLong(from?.amount) ? from.amount : Long.fromNumber(from?.amount as number || 0);

        // if the account amount is less than the amount to subtract; return insufficient funds
        if (fromAmount.lessThan(amountToDeduct)) {
            return { error: ErrInsufficientFunds() };
        }

        // for self-transfer, use same account data
        const isSelfTransfer = Buffer.from(fromKey).equals(Buffer.from(toKey));
        const toAccount = isSelfTransfer ? from : to;

        // get amounts as Long
        const newFromAmount = fromAmount.subtract(amountToDeduct);
        const toAmount = Long.isLong(toAccount?.amount) ? toAccount.amount : Long.fromNumber(toAccount?.amount as number || 0);
        const newToAmount = toAmount.add(msgAmount);
        const poolAmount = Long.isLong(feePool?.amount) ? feePool.amount : Long.fromNumber(feePool?.amount as number || 0);
        const newPoolAmount = poolAmount.add(feeAmount);

        // Update the accounts
        const updatedFrom = types.Account.create({ address: from?.address, amount: newFromAmount });
        const updatedTo = types.Account.create({ address: toAccount?.address, amount: newToAmount });
        const updatedPool = types.Pool.create({ id: feePool?.id, amount: newPoolAmount });

        // convert the accounts to bytes
        const newFromBytes = types.Account.encode(updatedFrom).finish();
        const newToBytes = types.Account.encode(updatedTo).finish();
        const newFeePoolBytes = types.Pool.encode(updatedPool).finish();

        // execute writes to the database
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        let writeResp: any;
        let writeErr: IPluginError | null;

        // if the from account is drained - delete the from account
        if (newFromAmount.isZero()) {
            [writeResp, writeErr] = await contract.plugin.StateWrite(contract, {
                sets: [
                    { key: feePoolKey, value: newFeePoolBytes },
                    { key: toKey, value: newToBytes },
                ],
                deletes: [{ key: fromKey }],
            });
        } else {
            [writeResp, writeErr] = await contract.plugin.StateWrite(contract, {
                sets: [
                    { key: feePoolKey, value: newFeePoolBytes },
                    { key: toKey, value: newToBytes },
                    { key: fromKey, value: newFromBytes },
                ],
            });
        }

        if (writeErr) {
            return { error: writeErr };
        }
        if (writeResp?.error) {
            return { error: writeResp.error };
        }

        return {};
    }

    // DeliverMessageReward() handles a 'reward' message (mints tokens to recipient, admin pays fee)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    static async DeliverMessageReward(contract: Contract, msg: any, fee: Long | number | undefined): Promise<any> {
        const adminQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));
        const recipientQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));
        const feeQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));

        // calculate keys
        const adminKey = KeyForAccount(msg.adminAddress!);
        const recipientKey = KeyForAccount(msg.recipientAddress!);
        const feePoolKey = KeyForFeePool(Long.fromNumber(contract.Config.ChainId));

        // read current state
        const [response, readErr] = await contract.plugin.StateRead(contract, {
            keys: [
                { queryId: feeQueryId, key: feePoolKey },
                { queryId: adminQueryId, key: adminKey },
                { queryId: recipientQueryId, key: recipientKey },
            ],
        });

        if (readErr) {
            return { error: readErr };
        }
        if (response?.error) {
            return { error: response.error };
        }

        // get bytes from response
        let adminBytes: Uint8Array | null = null;
        let recipientBytes: Uint8Array | null = null;
        let feePoolBytes: Uint8Array | null = null;

        for (const resp of response?.results || []) {
            const qid = resp.queryId as Long;
            if (qid.equals(adminQueryId)) {
                adminBytes = resp.entries?.[0]?.value || null;
            } else if (qid.equals(recipientQueryId)) {
                recipientBytes = resp.entries?.[0]?.value || null;
            } else if (qid.equals(feeQueryId)) {
                feePoolBytes = resp.entries?.[0]?.value || null;
            }
        }

        // unmarshal accounts
        const [adminRaw, adminErr] = Unmarshal(adminBytes || new Uint8Array(), types.Account);
        if (adminErr) {
            return { error: adminErr };
        }
        const [recipientRaw, recipientErr] = Unmarshal(recipientBytes || new Uint8Array(), types.Account);
        if (recipientErr) {
            return { error: recipientErr };
        }
        const [feePoolRaw, feePoolErr] = Unmarshal(feePoolBytes || new Uint8Array(), types.Pool);
        if (feePoolErr) {
            return { error: feePoolErr };
        }

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const admin = adminRaw as any;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const recipient = recipientRaw as any;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const feePool = feePoolRaw as any;

        const feeAmount = Long.isLong(fee) ? fee : Long.fromNumber(fee as number || 0);
        const adminAmount = Long.isLong(admin?.amount) ? admin.amount : Long.fromNumber(admin?.amount as number || 0);

        // admin must have enough to pay the fee
        if (adminAmount.lessThan(feeAmount)) {
            return { error: ErrInsufficientFunds() };
        }

        // apply state changes
        const msgAmount = Long.isLong(msg.amount) ? msg.amount : Long.fromNumber(msg.amount as number || 0);
        const newAdminAmount = adminAmount.subtract(feeAmount); // admin pays fee
        const recipientAmount = Long.isLong(recipient?.amount) ? recipient.amount : Long.fromNumber(recipient?.amount as number || 0);
        const newRecipientAmount = recipientAmount.add(msgAmount); // mint tokens to recipient
        const poolAmount = Long.isLong(feePool?.amount) ? feePool.amount : Long.fromNumber(feePool?.amount as number || 0);
        const newPoolAmount = poolAmount.add(feeAmount);

        // update accounts
        const updatedAdmin = types.Account.create({ address: admin?.address, amount: newAdminAmount });
        const updatedRecipient = types.Account.create({ address: recipient?.address || msg.recipientAddress, amount: newRecipientAmount });
        const updatedPool = types.Pool.create({ id: feePool?.id, amount: newPoolAmount });

        // marshal
        const newAdminBytes = types.Account.encode(updatedAdmin).finish();
        const newRecipientBytes = types.Account.encode(updatedRecipient).finish();
        const newFeePoolBytes = types.Pool.encode(updatedPool).finish();

        // write state changes
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        let writeResp: any;
        let writeErr: IPluginError | null;

        if (newAdminAmount.isZero()) {
            // delete drained admin account
            [writeResp, writeErr] = await contract.plugin.StateWrite(contract, {
                sets: [
                    { key: feePoolKey, value: newFeePoolBytes },
                    { key: recipientKey, value: newRecipientBytes },
                ],
                deletes: [{ key: adminKey }],
            });
        } else {
            [writeResp, writeErr] = await contract.plugin.StateWrite(contract, {
                sets: [
                    { key: feePoolKey, value: newFeePoolBytes },
                    { key: adminKey, value: newAdminBytes },
                    { key: recipientKey, value: newRecipientBytes },
                ],
            });
        }

        if (writeErr) {
            return { error: writeErr };
        }
        if (writeResp?.error) {
            return { error: writeResp.error };
        }

        return {};
    }

    // DeliverMessageFaucet() handles a 'faucet' message (mints tokens to recipient - no fee, no balance check)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    static async DeliverMessageFaucet(contract: Contract, msg: any): Promise<any> {
        const recipientQueryId = Long.fromNumber(Math.floor(Math.random() * Number.MAX_SAFE_INTEGER));

        const recipientKey = KeyForAccount(msg.recipientAddress!);

        // read current recipient state
        const [response, readErr] = await contract.plugin.StateRead(contract, {
            keys: [
                { queryId: recipientQueryId, key: recipientKey },
            ],
        });

        if (readErr) {
            return { error: readErr };
        }
        if (response?.error) {
            return { error: response.error };
        }

        // get recipient bytes
        let recipientBytes: Uint8Array | null = null;
        for (const resp of response?.results || []) {
            const qid = resp.queryId as Long;
            if (qid.equals(recipientQueryId)) {
                recipientBytes = resp.entries?.[0]?.value || null;
            }
        }

        // unmarshal recipient account (or create new if doesn't exist)
        const [recipientRaw, recipientErr] = Unmarshal(recipientBytes || new Uint8Array(), types.Account);
        if (recipientErr) {
            return { error: recipientErr };
        }

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const recipient = recipientRaw as any;

        // mint tokens to recipient
        const msgAmount = Long.isLong(msg.amount) ? msg.amount : Long.fromNumber(msg.amount as number || 0);
        const recipientAmount = Long.isLong(recipient?.amount) ? recipient.amount : Long.fromNumber(recipient?.amount as number || 0);
        const newRecipientAmount = recipientAmount.add(msgAmount);

        // update recipient
        const updatedRecipient = types.Account.create({ 
            address: recipient?.address || msg.recipientAddress, 
            amount: newRecipientAmount 
        });

        const newRecipientBytes = types.Account.encode(updatedRecipient).finish();

        // write state changes
        const [writeResp, writeErr] = await contract.plugin.StateWrite(contract, {
            sets: [
                { key: recipientKey, value: newRecipientBytes },
            ],
        });

        if (writeErr) {
            return { error: writeErr };
        }
        if (writeResp?.error) {
            return { error: writeResp.error };
        }

        return {};
    }
}

const accountPrefix = Buffer.from([1]); // store key prefix for accounts
const poolPrefix = Buffer.from([2]);    // store key prefix for pools
const paramsPrefix = Buffer.from([7]);  // store key prefix for governance parameters

// KeyForAccount() returns the state database key for an account
export function KeyForAccount(addr: Uint8Array): Uint8Array {
    return JoinLenPrefix(accountPrefix, Buffer.from(addr));
}

// KeyForFeeParams() returns the state database key for governance controlled 'fee parameters'
export function KeyForFeeParams(): Uint8Array {
    return JoinLenPrefix(paramsPrefix, Buffer.from("/f/"));
}

// KeyForFeePool() returns the state database key for governance controlled 'fee parameters'
export function KeyForFeePool(chainId: Long): Uint8Array {
    return JoinLenPrefix(poolPrefix, formatUint64(chainId));
}

function formatUint64(u: Long): Buffer {
    const b = Buffer.alloc(8);
    b.writeBigUInt64BE(BigInt(u.toString()));
    return b;
}
