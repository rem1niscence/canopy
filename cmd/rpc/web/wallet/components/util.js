import {OverlayTrigger, Toast, ToastContainer, Tooltip} from "react-bootstrap";

export function getFormInputs(type, account, validator) {
    let amount = type === "edit-stake" ? validator["staked_amount"] : null
    let netAddr = validator ? validator["net_address"] : ""
    let output = validator ? validator["output"] : ""
    let address = account != null ? account.address : ""
    address = type !== "send" && validator ? validator.address : address
    address = type === "stake" && validator.address ? "WARNING: validator already staked" : address
    let a = {
        privateKey: {
            "placeholder": "opt: private key hex to import",
            "defaultValue": "",
            "tooltip": "the raw private key to import if blank - will generate a new key",
            "label": "private_key",
            "inputText": "key",
            "feedback": "please choose a private key to import",
            "required": false,
            "type": "password",
            "minLength": 64,
            "maxLength": 128,
        },
        address: {
            "placeholder": "id of the node",
            "defaultValue": address,
            "tooltip": "required: the sender of the transaction",
            "label": "sender",
            "inputText": "address",
            "feedback": "please choose an address to send the transaction from",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        netAddr: {
            "placeholder": "url of the node",
            "defaultValue": netAddr,
            "tooltip": "required: the url of the validator for consensus and polling",
            "label": "net_address",
            "inputText": "net-addr",
            "feedback": "please choose a net address for the validator",
            "required": true,
            "type": "text",
            "minLength": 5,
            "maxLength": 50,
        },
        rec: {
            "placeholder": "recipient of the tx",
            "defaultValue": "",
            "tooltip": "required: the recipient of the transaction",
            "label": "recipient",
            "inputText": "recipient",
            "feedback": "please choose a recipient for the transaction",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        amount: {
            "placeholder": "amount value for the tx",
            "defaultValue": amount,
            "tooltip": "required: the amount of currency being sent / sold",
            "label": "amount",
            "inputText": "amount",
            "feedback": "please choose an amount for the tx",
            "required": true,
            "type": "number",
            "minLength": 1,
            "maxLength": 100,
        },
        receiveAmount: {
            "placeholder": "amount of counter asset to receive",
            "defaultValue": amount,
            "tooltip": "required: the amount of counter asset being received",
            "label": "receiveAmount",
            "inputText": "rec-amount",
            "feedback": "please choose a receive amount for the tx",
            "required": true,
            "type": "number",
            "minLength": 1,
            "maxLength": 100,
        },
        orderId: {
            "placeholder": "the id of the existing order",
            "tooltip": "required: the unique identifier of the order",
            "label": "orderId",
            "inputText": "order-id",
            "feedback": "please input an order id",
            "required": true,
            "type": "number",
            "minLength": 1,
            "maxLength": 100,
        },
        committeeId: {
            "placeholder": "the id of the committee / counter asset",
            "tooltip": "required: the unique identifier of the committee / counter asset",
            "label": "committeeId",
            "inputText": "commit-Id",
            "feedback": "please input a committeeId id",
            "required": true,
            "type": "number",
            "minLength": 1,
            "maxLength": 100,
        },
        receiveAddress: {
            "placeholder": "the address where the counter asset will be sent",
            "tooltip": "required: the sender of the transaction",
            "label": "receiveAddress",
            "inputText": "rec-addr",
            "feedback": "please choose an address to receive the counter asset to",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        output: {
            "placeholder": "output of the node",
            "defaultValue": output,
            "tooltip": "required: the non-custodial address where rewards and stake is directed to",
            "label": "output",
            "inputText": "output",
            "feedback": "please choose an output address for the validator",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        paramSpace: {
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the category 'space' of the parameter",
            "label": "param_space",
            "inputText": "param space",
            "feedback": "please choose a space for the parameter change",
            "required": true,
            "type": "select",
            "minLength": 1,
            "maxLength": 100,
        },
        paramKey: {
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the identifier of the parameter",
            "label": "param_key",
            "inputText": "param key",
            "feedback": "please choose a key for the parameter change",
            "required": true,
            "type": "select",
            "minLength": 1,
            "maxLength": 100,
        },
        paramValue: {
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the newly proposed value of the parameter",
            "label": "param_value",
            "inputText": "param val",
            "feedback": "please choose a value for the parameter change",
            "required": true,
            "type": "text",
            "minLength": 1,
            "maxLength": 100,
        },
        startBlock: {
            "placeholder": "1",
            "defaultValue": "",
            "tooltip": "required: the block when voting starts",
            "label": "start_block",
            "inputText": "start blk",
            "feedback": "please choose a height for start block",
            "required": true,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        endBlock: {
            "placeholder": "100",
            "defaultValue": "",
            "tooltip": "required: the block when voting is counted",
            "label": "end_block",
            "inputText": "end blk",
            "feedback": "please choose a height for end block",
            "required": true,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        memo: {
            "placeholder": "opt: note attached with the transaction",
            "defaultValue": "",
            "tooltip": "an optional note attached to the transaction - blank is recommended",
            "label": "memo",
            "inputText": "memo",
            "required": false,
            "minLength": 0,
            "maxLength": 200,
        },
        fee: {
            "placeholder": "opt: transaction fee",
            "defaultValue": "",
            "tooltip": " a small amount of CNPY deducted from the account to process any transaction blank = default fee",
            "label": "fee",
            "inputText": "txn-fee",
            "feedback": "please choose a valid number",
            "required": false,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        password: {
            "placeholder": "key password",
            "defaultValue": "",
            "tooltip": "the password for the private key sending the transaction",
            "label": "password",
            "inputText": "password",
            "feedback": "please choose a valid password",
            "required": true,
            "type": "password",
            "minLength": 0,
            "maxLength": 40,
        }
    }
    switch (type) {
        case "send":
            return [a.address, a.rec, a.amount, a.memo, a.fee, a.password]
        case "stake":
            return [a.address, a.netAddr, a.amount, a.output, a.memo, a.fee, a.password]
        case "create_order":
            return [a.address, a.committeeId, a.amount, a.receiveAmount, a.receiveAddress, a.memo, a.fee, a.password]
        case "edit_order":
            return [a.address, a.committeeId, a.orderId, a.amount, a.receiveAmount, a.receiveAddress, a.memo, a.fee, a.password]
        case "delete_order":
            return [a.address, a.committeeId, a.orderId, a.memo, a.fee, a.password]
        case "edit-stake":
            return [a.address, a.netAddr, a.amount, a.output, a.memo, a.fee, a.password]
        case "change-param":
            return [a.address, a.paramSpace, a.paramKey, a.paramValue, a.startBlock, a.endBlock, a.memo, a.fee, a.password]
        case "dao-transfer":
            return [a.address, a.amount, a.startBlock, a.endBlock, a.memo, a.fee, a.password]
        case "pass-and-addr":
            return [a.address, a.password]
        case "pass-and-pk":
            return [a.privateKey, a.password]
        case "pass-only":
            return [a.password]
        default:
            return [a.address, a.memo, a.fee, a.password]
    }
}

export const placeholders = {
    poll: {
        "PLACEHOLDER EXAMPLE": {
            "proposalHash": "PLACEHOLDER EXAMPLE",
            "proposalURL": "https://discord.com/channels/1310733928436600912/1323330593701761204",
            "accounts": {
                "approvedPercent": 38,
                "rejectPercent": 62,
                "votedPercent": 35
            },
            "validators": {
                "approvedPercent": 76,
                "rejectPercent": 24,
                "votedPercent": 77
            }
        }
    },
    pollJSON: {
        "proposal": "canopy network is the best",
        "endBlock": 100,
        "URL": "https://discord.com/link-to-thread"
    },
    proposals: {
        "2cbb73b8abdacf233f4c9b081991f1692145624a95004f496a95d3cce4d492a4": {
            "proposal": {
                "parameter_space": "cons|fee|val|gov",
                "parameter_key": "protocol_version",
                "parameter_value": "example",
                "start_height": 1,
                "end_height": 1000000,
                "signer": "4646464646464646464646464646464646464646464646464646464646464646"
            },
            "approve": false
        }
    },
    params: {
        "parameter_space": "consensus",
        "parameter_key": "protocol_version",
        "parameter_value": "1/150",
        "start_height": 1,
        "end_height": 100,
        "signer": "303739303732333263..."
    },
    rawTx: {
        "type": "change_parameter",
        "msg": {
            "parameter_space": "cons",
            "parameter_key": "block_size",
            "parameter_value": 1000,
            "start_height": 1,
            "end_height": 100,
            "signer": "1fe1e32edc41d688..."
        },
        "signature": {
            "public_key": "a88b9c0c7b77e7f8ac...",
            "signature": "8f6d016d04e350..."
        },
        "memo": "",
        "fee": 10000
    }

}

export function numberWithCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

export function formatNumber(nString, div = true, cutoff = 1000000000000000) {
    if (nString == null) {
        return "zero"
    }
    if (div) {
        nString /= 1000000
    }
    if (Number(nString) < cutoff) {
        return numberWithCommas(nString)
    }
    return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 8}).format(nString)
}

export function copy(state, setState, detail, toastText = "Copied!") {
    navigator.clipboard.writeText(detail)
    setState({...state, toast: toastText})
}

export function renderToast(state, setState) {
    return <ToastContainer id="toast" position={"bottom-end"}>
        <Toast bg={"dark"} onClose={() => setState({...state, toast: ""})} show={state.toast != ""} delay={2000} autohide>
            <Toast.Body>{state.toast}</Toast.Body>
        </Toast>
    </ToastContainer>
}

export function onFormSubmit(state, e, callback) {
    e.preventDefault()
    let r = {}
    for (let i = 0; ; i++) {
        if (!e.target[i] || !e.target[i].ariaLabel) {
            break
        }
        r[e.target[i].ariaLabel] = e.target[i].value
    }
    callback(r)
}

export function withTooltip(obj, text, key, dir = "right") {
    return <OverlayTrigger key={key} placement={dir} delay={{show: 250, hide: 400}}
                           overlay={<Tooltip id="button-tooltip">{text}</Tooltip>}
    >{obj}</OverlayTrigger>
}

export function getRatio(a, b) {
    if (a > b) {
        var bg = a;
        var sm = b;
    } else {
        var bg = b;
        var sm = a;
    }
    for (var i = 1; i < 1000000; i++) {
        var d = sm / i;
        var res = bg / d;
        var howClose = Math.abs(res - res.toFixed(0));
        if (howClose < .1) {
            if (a > b) {
                return res.toFixed(0) + ':' + i;
            } else {
                return i + ':' + res.toFixed(0);
            }
        }
    }
}

export function objEmpty(o) {
    if (!o) {
        return true
    }
    return Object.keys(o).length === 0
}