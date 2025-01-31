let rpcURL = "http://localhost:50002"; // default RPC URL
let adminRPCURL = "http://localhost:50003"; // default Admin RPC URL
let baseChainRPCURL = rpcURL; // default BaseChain RPC URL
let chainId = 1; // default chain id

if (typeof window !== "undefined" && window.__CONFIG__) {
  rpcURL = window.__CONFIG__.rpcURL;
  adminRPCURL = window.__CONFIG__.adminRPCURL;
  baseChainRPCURL = window.__CONFIG__.baseChainRPCURL;
  chainId = Number(window.__CONFIG__.chainId);
} else {
  console.log("config undefined");
}

export function getAdminRPCURL() {
  return adminRPCURL;
}

const keystorePath = "/v1/admin/keystore";
const keystoreGetPath = "/v1/admin/keystore-get";
const keystoreNewPath = "/v1/admin/keystore-new-key";
const keystoreImportPath = "/v1/admin/keystore-import-raw";
export const logsPath = "/v1/admin/log";
const resourcePath = "/v1/admin/resource-usage";
const txSendPath = "/v1/admin/tx-send";
const txStakePath = "/v1/admin/tx-stake";
const txEditStakePath = "/v1/admin/tx-edit-stake";
const txUnstakePath = "/v1/admin/tx-unstake";
const txPausePath = "/v1/admin/tx-pause";
const txUnpausePath = "/v1/admin/tx-unpause";
const txChangeParamPath = "/v1/admin/tx-change-param";
const txDaoTransfer = "/v1/admin/tx-dao-transfer";
const txCreateOrder = "/v1/admin/tx-create-order";
const txBuyOrder = "/v1/admin/tx-buy-order";
const txEditOrder = "/v1/admin/tx-edit-order";
const txDeleteOrder = "/v1/admin/tx-delete-order";
const txStartPoll = "/v1/admin/tx-start-poll";
const txVotePoll = "/v1/admin/tx-vote-poll";
export const consensusInfoPath = "/v1/admin/consensus-info?id=1";
export const configPath = "/v1/admin/config";
export const peerBookPath = "/v1/admin/peer-book";
export const peerInfoPath = "/v1/admin/peer-info";
const accountPath = "/v1/query/account";
const validatorPath = "/v1/query/validator";
const txsBySender = "/v1/query/txs-by-sender";
const txsByRec = "/v1/query/txs-by-rec";
const failedTxs = "/v1/query/failed-txs";
const pollPath = "/v1/gov/poll";
const proposalsPath = "/v1/gov/proposals";
const addVotePath = "/v1/gov/add-vote";
const delVotePath = "/v1/gov/del-vote";
const paramsPath = "/v1/query/params";
const orderPath = "/v1/query/order";
const txPath = "/v1/tx";
const height = "/v1/query/height";

export async function GET(url, path) {
  let resp = await fetch(url + path, {
    method: "GET",
  }).catch((rejected) => {
    console.log(rejected);
  });
  if (resp == null) {
    return {};
  }
  return resp.json();
}

export async function GETText(url, path) {
  let resp = await fetch(url + path, {
    method: "GET",
  }).catch((rejected) => {
    console.log(rejected);
  });
  if (resp == null) {
    return {};
  }
  return resp.text();
}

export async function POST(url, path, request) {
  let resp = await fetch(url + path, {
    method: "POST",
    body: request,
  }).catch((rejected) => {
    console.log(rejected);
  });
  if (resp == null) {
    return {};
  }
  return resp.json();
}

function heightAndAddrRequest(height, address) {
  return JSON.stringify({ height: height, address: address });
}

function pageAddrReq(page, addr) {
  return JSON.stringify({ pageNumber: page, address: addr, perPage: 5 });
}

function voteRequest(json, approve) {
  return JSON.stringify({ approve: approve, proposal: json });
}

function pollRequest(address, json, password, approve) {
  return JSON.stringify({ address: address, pollJSON: json, pollApprove: approve, password: password, submit: true });
}

function addressAndPwdRequest(address, password) {
  return JSON.stringify({ address: address, password: password, submit: true });
}

function pkAndPwdRequest(pk, password) {
  return JSON.stringify({ privateKey: pk, password: password });
}

function newTxRequest(
  address,
  pubKey,
  committees,
  netAddress,
  amount,
  delegate,
  earlyWithdrawal,
  output,
  signer,
  memo,
  fee,
  submit,
  password,
) {
  return JSON.stringify({
    address: address,
    pubKey: pubKey,
    netAddress: netAddress,
    committees: committees,
    amount: amount,
    delegate: delegate,
    earlyWithdrawal: earlyWithdrawal,
    output: output,
    signer: signer,
    memo: memo,
    fee: fee,
    submit: submit,
    password: password,
  });
}

function newSellOrderTxRequest(
  address,
  committeeId,
  orderId,
  sellAmount,
  receiveAmount,
  receiveAddress,
  memo,
  fee,
  submit,
  password,
) {
  return JSON.stringify({
    address: address,
    committees: committeeId.toString(),
    orderId: orderId,
    amount: sellAmount,
    receiveAmount: receiveAmount,
    receiveAddress: receiveAddress,
    memo: memo,
    fee: fee,
    submit: submit,
    password: password,
  });
}

function newBuyOrderTxRequest(address, receiveAddress, orderId, fee, submit, password) {
  return JSON.stringify({
    address: address,
    receiveAddress: receiveAddress,
    orderId: orderId,
    fee: fee,
    submit: submit,
    password: password,
  });
}

function newGovTxRequest(
  address,
  amount,
  paramSpace,
  paramKey,
  paramValue,
  startBlock,
  endBlock,
  memo,
  fee,
  submit,
  password,
) {
  return JSON.stringify({
    address: address,
    amount: amount,
    paramSpace: paramSpace,
    paramKey: paramKey,
    paramValue: paramValue,
    startBlock: startBlock,
    endBlock: endBlock,
    memo: memo,
    fee: fee,
    submit: submit,
    password: password,
  });
}

export async function Keystore() {
  return GET(adminRPCURL, keystorePath);
}

export async function KeystoreGet(address, password) {
  return POST(adminRPCURL, keystoreGetPath, addressAndPwdRequest(address, password));
}

export async function KeystoreNew(password) {
  return POST(adminRPCURL, keystoreNewPath, addressAndPwdRequest("", password));
}

export async function KeystoreImport(pk, password) {
  return POST(adminRPCURL, keystoreImportPath, pkAndPwdRequest(pk, password));
}

export async function Logs() {
  return GETText(adminRPCURL, logsPath);
}

export async function Account(height, address) {
  return POST(rpcURL, accountPath, heightAndAddrRequest(height, address));
}

export async function Poll() {
  return GET(rpcURL, pollPath);
}

export async function Proposals() {
  return GET(rpcURL, proposalsPath);
}

export async function AddVote(json, approve) {
  return POST(adminRPCURL, addVotePath, voteRequest(JSON.parse(json), approve));
}

export async function DelVote(json) {
  return POST(adminRPCURL, delVotePath, voteRequest(JSON.parse(json)));
}

export async function StartPoll(address, json, password) {
  return POST(adminRPCURL, txStartPoll, pollRequest(address, JSON.parse(json), password));
}

export async function VotePoll(address, json, approve, password) {
  return POST(adminRPCURL, txVotePoll, pollRequest(address, JSON.parse(json), password, approve));
}

export async function AccountWithTxs(height, address, page) {
  let result = {};
  result.account = await Account(height, address);

  const setStatus = (status) => (tx) => {
    tx.status = status;
  };

  result.sent_transactions = await TransactionsBySender(page, address);
  result.sent_transactions.results?.forEach(setStatus("included"));

  result.rec_transactions = await TransactionsByRec(page, address);
  result.rec_transactions.results?.forEach(setStatus("included"));

  result.failed_transactions = await FailedTransactions(page, address);
  result.failed_transactions.results?.forEach((tx) => {
    tx.status = "failure: ".concat(tx.error.Msg);
  });

  result.combined = (result.rec_transactions.results || [])
    .concat(result.sent_transactions.results || [])
    .concat(result.failed_transactions.results || []);

  result.combined.sort(function (a, b) {
    return a.transaction.time !== b.transaction.time
      ? b.transaction.time - a.transaction.time
      : a.height !== b.height
        ? b.height - a.height
        : b.index - a.index;
  });

  return result;
}

export function Height() {
  return POST(rpcURL, height);
}

export function TransactionsBySender(page, sender) {
  return POST(rpcURL, txsBySender, pageAddrReq(page, sender));
}

export function TransactionsByRec(page, rec) {
  return POST(rpcURL, txsByRec, pageAddrReq(page, rec));
}

export function FailedTransactions(page, sender) {
  return POST(rpcURL, failedTxs, pageAddrReq(page, sender));
}

export async function Validator(height, address) {
  return POST(rpcURL, validatorPath, heightAndAddrRequest(height, address));
}

export async function Resource() {
  return GET(adminRPCURL, resourcePath);
}

export async function TxSend(address, recipient, amount, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txSendPath,
    newTxRequest(address, "", "", "", amount, false, false, recipient, "", memo, Number(fee), submit, password),
  );
}

export async function TxStake(
  address,
  pubKey,
  committees,
  netAddress,
  amount,
  delegate,
  earlyWithdrawal,
  output,
  signer,
  memo,
  fee,
  password,
  submit,
) {
  return POST(
    adminRPCURL,
    txStakePath,
    newTxRequest(
      address,
      pubKey,
      committees,
      netAddress,
      amount,
      delegate.toLowerCase() === "true",
      earlyWithdrawal.toLowerCase() === "true",
      output,
      signer,
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxEditStake(
  address,
  committees,
  netAddress,
  amount,
  earlyWithdrawal,
  output,
  signer,
  memo,
  fee,
  password,
  submit,
) {
  return POST(
    adminRPCURL,
    txEditStakePath,
    newTxRequest(
      address,
      "",
      committees,
      netAddress,
      amount,
      false,
      earlyWithdrawal.toLowerCase() === "true",
      output,
      signer,
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxUnstake(address, signer, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txUnstakePath,
    newTxRequest(address, "", "", "", 0, false, false, "", signer, memo, Number(fee), submit, password),
  );
}

export async function TxPause(address, signer, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txPausePath,
    newTxRequest(address, "", "", "", 0, false, false, "", signer, memo, Number(fee), submit, password),
  );
}

export async function TxUnpause(address, signer, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txUnpausePath,
    newTxRequest(address, "", "", "", 0, false, false, "", signer, memo, Number(fee), submit, password),
  );
}

export async function TxChangeParameter(
  address,
  paramSpace,
  paramKey,
  paramValue,
  startBlock,
  endBlock,
  memo,
  fee,
  password,
  submit,
) {
  return POST(
    adminRPCURL,
    txChangeParamPath,
    newGovTxRequest(
      address,
      0,
      paramSpace,
      paramKey,
      paramValue,
      Number(startBlock),
      Number(endBlock),
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxDAOTransfer(address, amount, startBlock, endBlock, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txDaoTransfer,
    newGovTxRequest(
      address,
      Number(amount),
      "",
      "",
      "",
      Number(startBlock),
      Number(endBlock),
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxCreateOrder(
  address,
  committeeId,
  sellAmount,
  receiveAmount,
  receiveAddress,
  memo,
  fee,
  password,
  submit,
) {
  return POST(
    adminRPCURL,
    txCreateOrder,
    newSellOrderTxRequest(
      address,
      committeeId,
      0,
      Number(sellAmount),
      Number(receiveAmount),
      receiveAddress,
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxBuyOrder(address, receiveAddress, orderId, fee, password, submit) {
  return POST(
    adminRPCURL,
    txBuyOrder,
    newBuyOrderTxRequest(address, receiveAddress, Number(orderId), Number(fee), submit, password),
  );
}

export async function TxEditOrder(
  address,
  committeeId,
  orderId,
  sellAmount,
  receiveAmount,
  receiveAddress,
  memo,
  fee,
  password,
  submit,
) {
  return POST(
    adminRPCURL,
    txEditOrder,
    newSellOrderTxRequest(
      address,
      committeeId,
      Number(orderId),
      Number(sellAmount),
      Number(receiveAmount),
      receiveAddress,
      memo,
      Number(fee),
      submit,
      password,
    ),
  );
}

export async function TxDeleteOrder(address, committeeId, orderId, memo, fee, password, submit) {
  return POST(
    adminRPCURL,
    txDeleteOrder,
    newSellOrderTxRequest(address, committeeId, Number(orderId), 0, 0, "", memo, Number(fee), submit, password),
  );
}

export async function RawTx(json) {
  return POST(rpcURL, txPath, json);
}

export async function Params(height) {
  return POST(rpcURL, paramsPath, heightAndAddrRequest(height, ""));
}

export async function ConsensusInfo() {
  return GET(adminRPCURL, consensusInfoPath);
}

export async function PeerInfo() {
  return GET(adminRPCURL, peerInfoPath);
}
