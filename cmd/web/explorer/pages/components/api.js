const rpcURL = "http://127.0.0.1:6001"

const blocksPath = "/v1/query/blocks"
const blockByHashPath = "/v1/query/block-by-hash"
const blockByHeightPath = "/v1/query/block-by-height"
const txByHashPath = "/v1/query/tx-by-hash"
const txsBySender = "/v1/query/txs-by-sender"
const txsByRec = "/v1/query/txs-by-rec"
const txsByHeightPath = "/v1/query/txs-by-height"
const pendingPath = "/v1/query/pending"
const validatorsPath = "/v1/query/validators"
const consValidatorsPath = "/v1/query/cons-validators"
const accountsPath = "/v1/query/accounts"
const poolPath = "/v1/query/pool"
const accountPath = "/v1/query/account"
const validatorPath = "/v1/query/validator"
const paramsPath = "/v1/query/params"
const supplyPath = "/v1/query/supply"


function heightRequest(height) {
    return `{"height":` + height + `}`
}

function hashRequest(hash) {
    return `{"hash":"` + hash + `"}`
}

function pageAddrReq(page, addr) {
    return `{"address":"` + addr + `", "pageNumber":` + page + `, "perPage":10}`
}

function heightAndAddrRequest(height, address) {
    return `{"height":` + height + `, "address":"` + address + `"}`
}

function heightAndNameRequest(height, name) {
    return `{"height":` + height + `, "name":"` + name + `"}`
}

function pageHeightReq(page, height) {
    return `{"height":` + height + `, "pageNumber":` + page + `, "perPage":10}`
}

export async function GET(path, callback) {
    fetch(rpcURL + path, {
        method: 'GET',
    })
        .then((response) => response.json())
        .then((data) => callback(data))
        .catch(rejected => {
            console.log(rejected);
        });
}

export async function POST(request, path) {
    let resp = await fetch(rpcURL + path, {
        method: 'POST',
        body: request,
    })
        .catch(rejected => {
            console.log(rejected);
        });
    if (resp == null) {
        return {}
    }
    return resp.json()
}

export function Blocks(page, _) {
    return POST(pageHeightReq(page, 0), blocksPath)
}

export function Transactions(page, height) {
    return POST(pageHeightReq(page, height), txsByHeightPath)
}

export function Accounts(page, _) {
    return POST(pageHeightReq(page, 0), accountsPath)
}

export function Validators(page, _) {
    return POST(pageHeightReq(page, 0), validatorsPath)
}

export function ConsValidators(page, _) {
    return POST(pageHeightReq(page, 0), consValidatorsPath)
}

export function DAO(height, _) {
    return POST(heightAndNameRequest(height, "DAO"), poolPath)
}

export function Account(height, address) {
    return POST(heightAndAddrRequest(height, address), accountPath)
}

export async function AccountWithTxs(height, address, page) {
    let result = {}
    result.account = await Account(height, address)
    result.sent_transactions = await TransactionsBySender(page, address)
    result.rec_transactions = await TransactionsByRec(page, address)
    return result
}

export function Params(height, _) {
    return POST(heightRequest(height), paramsPath)
}

export function Supply(height, _) {
    return POST(heightRequest(height), supplyPath)
}

export function Validator(height, address) {
    return POST(heightAndAddrRequest(height, address), validatorPath)
}

export function BlockByHeight(height) {
    return POST(heightRequest(height), blockByHeightPath)
}

export function BlockByHash(hash) {
    return POST(hashRequest(hash), blockByHashPath)
}

export function TxByHash(hash) {
    return POST(hashRequest(hash), txByHashPath)
}

export function TransactionsBySender(page, sender) {
    return POST(pageAddrReq(page, sender), txsBySender)
}

export function TransactionsByRec(page, rec) {
    return POST(pageAddrReq(page, rec), txsByRec)
}

export function Pending(page, _) {
    return POST(pageAddrReq(page, ""), pendingPath)
}