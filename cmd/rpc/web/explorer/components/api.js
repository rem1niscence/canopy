const rpcURL = "http://127.0.0.1:50002"

// RPC PATHS BELOW

const blocksPath = "/v1/query/blocks"
const blockByHashPath = "/v1/query/block-by-hash"
const blockByHeightPath = "/v1/query/block-by-height"
const txByHashPath = "/v1/query/tx-by-hash"
const txsBySender = "/v1/query/txs-by-sender"
const txsByRec = "/v1/query/txs-by-rec"
const txsByHeightPath = "/v1/query/txs-by-height"
const pendingPath = "/v1/query/pending"
const validatorsPath = "/v1/query/validators"
const accountsPath = "/v1/query/accounts"
const poolPath = "/v1/query/pool"
const accountPath = "/v1/query/account"
const validatorPath = "/v1/query/validator"
const paramsPath = "/v1/query/params"
const supplyPath = "/v1/query/supply"
const ordersPath = "/v1/query/orders"

// POST

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

// REQUEST OBJECTS BELOW

function heightRequest(height) {
    return JSON.stringify({height: height})
}

function hashRequest(hash) {
    return JSON.stringify({hash: hash})
}

function pageAddrReq(page, addr) {
    return JSON.stringify({pageNumber: page, perPage: 10, address: addr})
}

function heightAndAddrRequest(height, address) {
    return JSON.stringify({height: height, address: address})
}

function heightAndIDRequest(height, id) {
    return JSON.stringify({height: height, id: id})
}

function pageHeightReq(page, height) {
    return JSON.stringify({pageNumber: page, perPage: 10, height: height})
}

function validatorsReq(page, height, committee) {
    return JSON.stringify({height: height, pageNumber: page, perPage: 10, committee: committee})
}

// API CALLS BELOW

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

export function Committee(page, committee_id) {
    return POST(validatorsReq(page, 0, committee_id), validatorsPath)
}

export function DAO(height, _) {
    return POST(heightAndIDRequest(height, 4294967296), poolPath)
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

export function Orders(committee_id) {
    return POST(heightAndIDRequest(0, committee_id), ordersPath)
}

// COMPONENT SPECIFIC API CALLS BELOW

// getModalData() executes API call(s) and prepares data for the modal component based on the search type
export async function getModalData(query, page) {
    const noResult = "no result found";

    // Handle string query cases
    if (typeof query === "string") {
        // Block by hash
        if (query.length === 64) {
            const block = await BlockByHash(query);
            if (block?.block_header?.hash) return {"block": block};

            const tx = await TxByHash(query);
            return tx?.sender ? tx : noResult;
        }

        // Validator or account by address
        if (query.length === 40) {
            const [val, acc] = await Promise.all([Validator(0, query), AccountWithTxs(0, query, page)]);
            if (!acc.account.address && !val.address) return noResult;
            return acc.account.address ? {...acc, validator: val} : {"validator": val};
        }

        return noResult;
    }

    // Handle block by height
    const block = await BlockByHeight(query);
    return block?.block_header?.hash ? {"block": block} : noResult;
}

// getCardData() executes api calls and prepares the data for the cards
export async function getCardData() {
    let cardData = {}
    cardData.blocks = await Blocks(1, 0)
    cardData.canopyCommittee = await Committee(1, 1)
    cardData.supply = await Supply(0, 0)
    cardData.pool = await DAO(0, 0)
    cardData.params = await Params(0, 0)
    return cardData
}

// getTableData() executes an api call for the table based on the page and category
export async function getTableData(page, category, committee) {
    switch (category) {
        case 0:
            return await Blocks(page, 0)
        case 1:
            return await Transactions(page, 0)
        case 2:
            return await Pending(page, 0)
        case 3:
            return await Accounts(page, 0)
        case 4:
            return await Validators(page, 0)
        case 5:
            return await Params(page, 0)
        case 6:
            return await Orders(committee)
        case 7:
            return await Supply(0)
    }
}