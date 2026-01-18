using System;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using Google.Protobuf.WellKnownTypes;
using Types;

namespace CanopyPlugin
{
    // ContractConfig: the configuration of the contract
    public static class ContractConfig
    {
        public const string Name = "csharp_plugin_contract";
        public const int Id = 1;
        public const int Version = 1;
        public static readonly string[] SupportedTransactions = { "send", "reward", "faucet" };
        public static readonly string[] TransactionTypeUrls = 
        { 
            "type.googleapis.com/types.MessageSend",
            "type.googleapis.com/types.MessageReward",
            "type.googleapis.com/types.MessageFaucet"
        };
        public static readonly string[] EventTypeUrls = Array.Empty<string>();
        // Include google/protobuf/any.proto first as it's a dependency of event.proto and tx.proto
        public static readonly ByteString[] FileDescriptorProtos =
        {
            ByteString.CopyFrom(Any.Descriptor.File.ToProto().ToByteArray()),
            ByteString.CopyFrom(AccountReflection.Descriptor.ToProto().ToByteArray()),
            ByteString.CopyFrom(EventReflection.Descriptor.ToProto().ToByteArray()),
            ByteString.CopyFrom(PluginReflection.Descriptor.ToProto().ToByteArray()),
            ByteString.CopyFrom(TxReflection.Descriptor.ToProto().ToByteArray()),
        };
    }

    // Contract defines the smart contract that implements the extended logic of the nested chain
    public class Contract
    {
        private static readonly Random Random = new();

        public Config Config { get; }
        public PluginFSMConfig? FsmConfig { get; }
        public Plugin Plugin { get; }
        public ulong FsmId { get; }

        public Contract(Config config, Plugin plugin, ulong fsmId, PluginFSMConfig? fsmConfig = null)
        {
            Config = config;
            Plugin = plugin;
            FsmId = fsmId;
            FsmConfig = fsmConfig;
        }

        // Genesis implements logic to import a json file to create the state at height 0
        public PluginGenesisResponse Genesis(PluginGenesisRequest request)
        {
            return new PluginGenesisResponse();
        }

        // BeginBlock is code that is executed at the start of applying the block
        public PluginBeginResponse BeginBlock(PluginBeginRequest request)
        {
            return new PluginBeginResponse();
        }

        // CheckTx is code that is executed to statelessly validate a transaction
        public async Task<PluginCheckResponse> CheckTxAsync(PluginCheckRequest request)
        {
            // validate fee
            var resp = await Plugin.StateReadAsync(this, new PluginStateReadRequest
            {
                Keys = { new PluginKeyRead { QueryId = (ulong)Random.NextInt64(), Key = ByteString.CopyFrom(KeyForFeeParams()) } }
            });

            if (resp.Error != null)
            {
                return new PluginCheckResponse { Error = resp.Error };
            }

            // convert bytes into fee parameters
            var minFees = Unmarshal<FeeParams>(resp.Results[0].Entries[0].Value.ToByteArray());
            if (minFees == null)
            {
                return new PluginCheckResponse { Error = ErrUnmarshal("fee params") };
            }

            // check for the minimum fee
            if (request.Tx.Fee < minFees.SendFee)
            {
                return new PluginCheckResponse { Error = ErrTxFeeBelowStateLimit() };
            }

            // handle the message based on type
            var typeUrl = request.Tx.Msg.TypeUrl;
            
            if (typeUrl.EndsWith("/types.MessageSend"))
            {
                var msg = new MessageSend();
                msg.MergeFrom(request.Tx.Msg.Value);
                return CheckMessageSend(msg);
            }
            else if (typeUrl.EndsWith("/types.MessageReward"))
            {
                var msg = new MessageReward();
                msg.MergeFrom(request.Tx.Msg.Value);
                return CheckMessageReward(msg);
            }
            else if (typeUrl.EndsWith("/types.MessageFaucet"))
            {
                var msg = new MessageFaucet();
                msg.MergeFrom(request.Tx.Msg.Value);
                return CheckMessageFaucet(msg);
            }
            else
            {
                return new PluginCheckResponse { Error = ErrInvalidMessageCast() };
            }
        }

        // DeliverTx is code that is executed to apply a transaction
        public async Task<PluginDeliverResponse> DeliverTxAsync(PluginDeliverRequest request)
        {
            // handle the message based on type
            var typeUrl = request.Tx.Msg.TypeUrl;
            
            if (typeUrl.EndsWith("/types.MessageSend"))
            {
                var msg = new MessageSend();
                msg.MergeFrom(request.Tx.Msg.Value);
                return await DeliverMessageSendAsync(msg, request.Tx.Fee);
            }
            else if (typeUrl.EndsWith("/types.MessageReward"))
            {
                var msg = new MessageReward();
                msg.MergeFrom(request.Tx.Msg.Value);
                return await DeliverMessageRewardAsync(msg, request.Tx.Fee);
            }
            else if (typeUrl.EndsWith("/types.MessageFaucet"))
            {
                var msg = new MessageFaucet();
                msg.MergeFrom(request.Tx.Msg.Value);
                return await DeliverMessageFaucetAsync(msg);
            }
            else
            {
                return new PluginDeliverResponse { Error = ErrInvalidMessageCast() };
            }
        }

        // EndBlock is code that is executed at the end of applying a block
        public PluginEndResponse EndBlock(PluginEndRequest request)
        {
            return new PluginEndResponse();
        }

        // CheckMessageSend statelessly validates a 'send' message
        private PluginCheckResponse CheckMessageSend(MessageSend msg)
        {
            // check sender address
            if (msg.FromAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // check recipient address
            if (msg.ToAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // check amount
            if (msg.Amount == 0)
            {
                return new PluginCheckResponse { Error = ErrInvalidAmount() };
            }

            // return the authorized signers
            return new PluginCheckResponse
            {
                Recipient = msg.ToAddress,
                AuthorizedSigners = { msg.FromAddress }
            };
        }

        // CheckMessageReward statelessly validates a 'reward' message
        private PluginCheckResponse CheckMessageReward(MessageReward msg)
        {
            // validate admin address (must be 20 bytes)
            if (msg.AdminAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // validate recipient address
            if (msg.RecipientAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // validate amount
            if (msg.Amount == 0)
            {
                return new PluginCheckResponse { Error = ErrInvalidAmount() };
            }

            // return authorized signers (admin must sign this tx)
            return new PluginCheckResponse
            {
                Recipient = msg.RecipientAddress,
                AuthorizedSigners = { msg.AdminAddress }
            };
        }

        // CheckMessageFaucet statelessly validates a 'faucet' message
        private PluginCheckResponse CheckMessageFaucet(MessageFaucet msg)
        {
            // validate signer address (must be 20 bytes)
            if (msg.SignerAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // validate recipient address
            if (msg.RecipientAddress.Length != 20)
            {
                return new PluginCheckResponse { Error = ErrInvalidAddress() };
            }

            // validate amount
            if (msg.Amount == 0)
            {
                return new PluginCheckResponse { Error = ErrInvalidAmount() };
            }

            // return authorized signers (signer must sign this tx)
            return new PluginCheckResponse
            {
                Recipient = msg.RecipientAddress,
                AuthorizedSigners = { msg.SignerAddress }
            };
        }

        // DeliverMessageSend handles a 'send' message
        private async Task<PluginDeliverResponse> DeliverMessageSendAsync(MessageSend msg, ulong fee)
        {
            var fromQueryId = (ulong)Random.NextInt64();
            var toQueryId = (ulong)Random.NextInt64();
            var feeQueryId = (ulong)Random.NextInt64();

            // calculate the from key and to key
            var fromKey = KeyForAccount(msg.FromAddress.ToByteArray());
            var toKey = KeyForAccount(msg.ToAddress.ToByteArray());
            var feePoolKey = KeyForFeePool((ulong)Config.ChainId);

            // get the from and to account
            var response = await Plugin.StateReadAsync(this, new PluginStateReadRequest
            {
                Keys =
                {
                    new PluginKeyRead { QueryId = feeQueryId, Key = ByteString.CopyFrom(feePoolKey) },
                    new PluginKeyRead { QueryId = fromQueryId, Key = ByteString.CopyFrom(fromKey) },
                    new PluginKeyRead { QueryId = toQueryId, Key = ByteString.CopyFrom(toKey) }
                }
            });

            // check for internal error
            if (response.Error != null)
            {
                return new PluginDeliverResponse { Error = response.Error };
            }

            // get the bytes from response
            byte[]? fromBytes = null, toBytes = null, feePoolBytes = null;
            foreach (var result in response.Results)
            {
                if (result.QueryId == fromQueryId)
                    fromBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
                else if (result.QueryId == toQueryId)
                    toBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
                else if (result.QueryId == feeQueryId)
                    feePoolBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
            }

            // convert the bytes to account structures
            var from = new Account();
            var to = new Account();
            var feePool = new Pool();

            if (fromBytes != null && fromBytes.Length > 0)
                from.MergeFrom(fromBytes);
            if (toBytes != null && toBytes.Length > 0)
                to.MergeFrom(toBytes);
            if (feePoolBytes != null && feePoolBytes.Length > 0)
                feePool.MergeFrom(feePoolBytes);

            // add fee to 'amount to deduct'
            var amountToDeduct = msg.Amount + fee;

            // if the account amount is less than the amount to subtract; return insufficient funds
            if (from.Amount < amountToDeduct)
            {
                return new PluginDeliverResponse { Error = ErrInsufficientFunds() };
            }

            // for self-transfer, use same account data
            var isSelfTransfer = fromKey.SequenceEqual(toKey);
            if (isSelfTransfer)
            {
                to = from;
            }

            // subtract from sender
            from.Amount -= amountToDeduct;
            // add the fee to the 'fee pool'
            feePool.Amount += fee;
            // add to recipient
            to.Amount += msg.Amount;

            // execute writes to the database
            var writeRequest = new PluginStateWriteRequest();

            // add fee pool update
            writeRequest.Sets.Add(new PluginSetOp
            {
                Key = ByteString.CopyFrom(feePoolKey),
                Value = ByteString.CopyFrom(from.Amount == 0 ? to.ToByteArray() : feePool.ToByteArray())
            });

            // fix: always write fee pool correctly
            writeRequest.Sets.Clear();
            writeRequest.Sets.Add(new PluginSetOp
            {
                Key = ByteString.CopyFrom(feePoolKey),
                Value = ByteString.CopyFrom(feePool.ToByteArray())
            });

            // if the from account is drained - delete the from account
            if (from.Amount == 0)
            {
                writeRequest.Deletes.Add(new PluginDeleteOp { Key = ByteString.CopyFrom(fromKey) });
            }
            else
            {
                writeRequest.Sets.Add(new PluginSetOp
                {
                    Key = ByteString.CopyFrom(fromKey),
                    Value = ByteString.CopyFrom(from.ToByteArray())
                });
            }

            // write to account (skip if self-transfer since we already handled it)
            if (!isSelfTransfer)
            {
                writeRequest.Sets.Add(new PluginSetOp
                {
                    Key = ByteString.CopyFrom(toKey),
                    Value = ByteString.CopyFrom(to.ToByteArray())
                });
            }

            var writeResp = await Plugin.StateWriteAsync(this, writeRequest);
            return new PluginDeliverResponse { Error = writeResp.Error };
        }

        // DeliverMessageReward handles a 'reward' message (mints tokens to recipient)
        private async Task<PluginDeliverResponse> DeliverMessageRewardAsync(MessageReward msg, ulong fee)
        {
            var adminQueryId = (ulong)Random.NextInt64();
            var recipientQueryId = (ulong)Random.NextInt64();
            var feeQueryId = (ulong)Random.NextInt64();

            // calculate state keys
            var adminKey = KeyForAccount(msg.AdminAddress.ToByteArray());
            var recipientKey = KeyForAccount(msg.RecipientAddress.ToByteArray());
            var feePoolKey = KeyForFeePool((ulong)Config.ChainId);

            // read current state
            var response = await Plugin.StateReadAsync(this, new PluginStateReadRequest
            {
                Keys =
                {
                    new PluginKeyRead { QueryId = feeQueryId, Key = ByteString.CopyFrom(feePoolKey) },
                    new PluginKeyRead { QueryId = adminQueryId, Key = ByteString.CopyFrom(adminKey) },
                    new PluginKeyRead { QueryId = recipientQueryId, Key = ByteString.CopyFrom(recipientKey) }
                }
            });

            // check for internal error
            if (response.Error != null)
            {
                return new PluginDeliverResponse { Error = response.Error };
            }

            // parse results by QueryId
            byte[]? adminBytes = null, recipientBytes = null, feePoolBytes = null;
            foreach (var result in response.Results)
            {
                if (result.QueryId == adminQueryId)
                    adminBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
                else if (result.QueryId == recipientQueryId)
                    recipientBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
                else if (result.QueryId == feeQueryId)
                    feePoolBytes = result.Entries.FirstOrDefault()?.Value?.ToByteArray();
            }

            // unmarshal accounts
            var admin = new Account();
            var recipient = new Account();
            var feePool = new Pool();

            if (adminBytes != null && adminBytes.Length > 0)
                admin.MergeFrom(adminBytes);
            if (recipientBytes != null && recipientBytes.Length > 0)
                recipient.MergeFrom(recipientBytes);
            if (feePoolBytes != null && feePoolBytes.Length > 0)
                feePool.MergeFrom(feePoolBytes);

            // admin must have enough to pay the fee
            if (admin.Amount < fee)
            {
                return new PluginDeliverResponse { Error = ErrInsufficientFunds() };
            }

            // apply state changes
            admin.Amount -= fee;           // admin pays fee
            recipient.Amount += msg.Amount; // mint tokens to recipient
            feePool.Amount += fee;

            // execute writes to the database
            var writeRequest = new PluginStateWriteRequest();

            // always write fee pool
            writeRequest.Sets.Add(new PluginSetOp
            {
                Key = ByteString.CopyFrom(feePoolKey),
                Value = ByteString.CopyFrom(feePool.ToByteArray())
            });

            // if the admin account is drained - delete the admin account
            if (admin.Amount == 0)
            {
                writeRequest.Deletes.Add(new PluginDeleteOp { Key = ByteString.CopyFrom(adminKey) });
            }
            else
            {
                writeRequest.Sets.Add(new PluginSetOp
                {
                    Key = ByteString.CopyFrom(adminKey),
                    Value = ByteString.CopyFrom(admin.ToByteArray())
                });
            }

            // write recipient account
            writeRequest.Sets.Add(new PluginSetOp
            {
                Key = ByteString.CopyFrom(recipientKey),
                Value = ByteString.CopyFrom(recipient.ToByteArray())
            });

            var writeResp = await Plugin.StateWriteAsync(this, writeRequest);
            return new PluginDeliverResponse { Error = writeResp.Error };
        }

        // DeliverMessageFaucet handles a 'faucet' message (mints tokens to recipient - no fee, no balance check)
        private async Task<PluginDeliverResponse> DeliverMessageFaucetAsync(MessageFaucet msg)
        {
            var recipientQueryId = (ulong)Random.NextInt64();

            // calculate state key for recipient
            var recipientKey = KeyForAccount(msg.RecipientAddress.ToByteArray());

            // read current recipient state
            var response = await Plugin.StateReadAsync(this, new PluginStateReadRequest
            {
                Keys =
                {
                    new PluginKeyRead { QueryId = recipientQueryId, Key = ByteString.CopyFrom(recipientKey) }
                }
            });

            // check for internal error
            if (response.Error != null)
            {
                return new PluginDeliverResponse { Error = response.Error };
            }

            // get recipient bytes
            byte[]? recipientBytes = null;
            foreach (var result in response.Results)
            {
                if (result.QueryId == recipientQueryId && result.Entries.Count > 0)
                {
                    recipientBytes = result.Entries[0].Value?.ToByteArray();
                }
            }

            // unmarshal recipient account (or create new if doesn't exist)
            var recipient = new Account();
            if (recipientBytes != null && recipientBytes.Length > 0)
            {
                recipient.MergeFrom(recipientBytes);
            }

            // mint tokens to recipient
            recipient.Amount += msg.Amount;

            // write state changes
            var writeRequest = new PluginStateWriteRequest();
            writeRequest.Sets.Add(new PluginSetOp
            {
                Key = ByteString.CopyFrom(recipientKey),
                Value = ByteString.CopyFrom(recipient.ToByteArray())
            });

            var writeResp = await Plugin.StateWriteAsync(this, writeRequest);
            return new PluginDeliverResponse { Error = writeResp.Error };
        }

        // State key prefixes
        private static readonly byte[] AccountPrefix = { 0x01 };
        private static readonly byte[] PoolPrefix = { 0x02 };
        private static readonly byte[] ParamsPrefix = { 0x07 };

        // KeyForAccount returns the state database key for an account
        public static byte[] KeyForAccount(byte[] addr)
        {
            return JoinLenPrefix(AccountPrefix, addr);
        }

        // KeyForFeeParams returns the state database key for fee parameters
        public static byte[] KeyForFeeParams()
        {
            return JoinLenPrefix(ParamsPrefix, Encoding.UTF8.GetBytes("/f/"));
        }

        // KeyForFeePool returns the state database key for the fee pool
        public static byte[] KeyForFeePool(ulong chainId)
        {
            return JoinLenPrefix(PoolPrefix, FormatUInt64(chainId));
        }

        // JoinLenPrefix appends the items together separated by a single byte to represent the length
        public static byte[] JoinLenPrefix(params byte[][] items)
        {
            var result = new System.Collections.Generic.List<byte>();
            foreach (var item in items)
            {
                if (item == null || item.Length == 0)
                    continue;
                result.Add((byte)item.Length);
                result.AddRange(item);
            }
            return result.ToArray();
        }

        // FormatUInt64 converts a ulong to big-endian bytes
        public static byte[] FormatUInt64(ulong value)
        {
            var bytes = BitConverter.GetBytes(value);
            if (BitConverter.IsLittleEndian)
                Array.Reverse(bytes);
            return bytes;
        }

        // Marshal serializes a proto.Message into a byte slice
        public static byte[] Marshal(IMessage message)
        {
            return message.ToByteArray();
        }

        // Unmarshal deserializes a byte slice into a proto.Message
        public static T? Unmarshal<T>(byte[] data) where T : IMessage<T>, new()
        {
            if (data == null || data.Length == 0)
                return default;
            var parser = new MessageParser<T>(() => new T());
            return parser.ParseFrom(data);
        }

        // Error factory methods - matching Go implementation
        private const string DefaultModule = "plugin";

        public static PluginError ErrPluginTimeout() =>
            new() { Code = 1, Module = DefaultModule, Msg = "a plugin timeout occurred" };

        public static PluginError ErrMarshal(string err) =>
            new() { Code = 2, Module = DefaultModule, Msg = $"marshal() failed with err: {err}" };

        public static PluginError ErrUnmarshal(string err) =>
            new() { Code = 3, Module = DefaultModule, Msg = $"unmarshal() failed with err: {err}" };

        public static PluginError ErrFailedPluginRead(string err) =>
            new() { Code = 4, Module = DefaultModule, Msg = $"a plugin read failed with err: {err}" };

        public static PluginError ErrFailedPluginWrite(string err) =>
            new() { Code = 5, Module = DefaultModule, Msg = $"a plugin write failed with err: {err}" };

        public static PluginError ErrInvalidPluginRespId() =>
            new() { Code = 6, Module = DefaultModule, Msg = "plugin response id is invalid" };

        public static PluginError ErrUnexpectedFSMToPlugin(string type) =>
            new() { Code = 7, Module = DefaultModule, Msg = $"unexpected FSM to plugin: {type}" };

        public static PluginError ErrInvalidFSMToPluginMessage(string type) =>
            new() { Code = 8, Module = DefaultModule, Msg = $"invalid FSM to plugin: {type}" };

        public static PluginError ErrInsufficientFunds() =>
            new() { Code = 9, Module = DefaultModule, Msg = "insufficient funds" };

        public static PluginError ErrFromAny(string err) =>
            new() { Code = 10, Module = DefaultModule, Msg = $"fromAny() failed with err: {err}" };

        public static PluginError ErrInvalidMessageCast() =>
            new() { Code = 11, Module = DefaultModule, Msg = "the message cast failed" };

        public static PluginError ErrInvalidAddress() =>
            new() { Code = 12, Module = DefaultModule, Msg = "address is invalid" };

        public static PluginError ErrInvalidAmount() =>
            new() { Code = 13, Module = DefaultModule, Msg = "amount is invalid" };

        public static PluginError ErrTxFeeBelowStateLimit() =>
            new() { Code = 14, Module = DefaultModule, Msg = "tx.fee is below state limit" };
    }
}
