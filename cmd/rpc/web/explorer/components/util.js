import Tooltip from "react-bootstrap/Tooltip";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import React from 'react';
import {Pagination} from "react-bootstrap";

export function formatNumberWCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

export function formatNumber(nString, cutoff = 1000000) {
    if (Number(nString) < cutoff) {
        return formatNumberWCommas(nString)
    }
    return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 3}).format(nString)
}

function string_to_milliseconds(str) {
    const units = {
        yr: {re: /(\d+|\d*\.\d+)\s*(Y)/i, s: 1000 * 60 * 60 * 24 * 365}, // Case insensitive "Y"
        mo: {re: /(\d+|\d*\.\d+)\s*(M|[Mm][Oo])/, s: 1000 * 60 * 60 * 24 * 30}, // Case insensitive "mo" or sensitive "M"
        wk: {re: /(\d+|\d*\.\d+)\s*(W)/i, s: 1000 * 60 * 60 * 24 * 7}, // Case insensitive "W"
        dy: {re: /(\d+|\d*\.\d+)\s*(D)/i, s: 1000 * 60 * 60 * 24}, // Case insensitive "D"
        hr: {re: /(\d+|\d*\.\d+)\s*(h)/i, s: 1000 * 60 * 60}, // Case insensitive "h"
        mn: {re: /(\d+|\d*\.\d+)\s*(m(?![so])|[Mm][Ii]?[Nn])/, s: 1000 * 60}, // Case insensitive "min"/"mn" or sensitive "m" (if not followed by "s" or "o")
        sc: {re: /(\d+|\d*\.\d+)\s*(s)/i, s: 1000}, // Case insensitive "s"
        ms: {re: /(\d+|\d*\.\d+)\s*(ms|mil)/i, s: 1}, // Case insensitive "ms"/"mil"
    };
    return Object.values(units).reduce((sum, unit) => sum + (str.match(unit.re)?.[1] * unit.s || 0), 0);
}

Date.prototype.addMS = function (s) {
    this.setTime(this.getTime() + (s));
    return this;
}

export function addDate(value, duration) {
    const milliseconds = Math.floor(value / 1000)
    const date = new Date(milliseconds)
    let ms = string_to_milliseconds(duration)
    return date.addMS(ms).toLocaleTimeString()
}

export function formatBytes(a, b = 2) {
    if (!+a) return "0 Bytes";
    const c = 0 > b ? 0 : b, d = Math.floor(Math.log(a) / Math.log(1024));
    return `${parseFloat((a / Math.pow(1024, d)).toFixed(c))} ${["B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"][d]}`
}

export function formatTime(value) {
    const date = new Date(Math.floor(value / 1000))
    console.log(date.toLocaleTimeString())
    return date.toLocaleTimeString()
}

export function formatIfTime(key, value) {
    if (key.includes("time")) {
        return formatTime(value)
    }
    return value
}

export function convertIfNumber(str) {
    if (!isNaN(str) && !isNaN(parseFloat(str))) {
        return Number(str)
    } else {
        return str
    }
}

export function withTooltip(obj, text, key, dir = "right") {
    return <OverlayTrigger key={key} placement={dir} delay={{show: 250, hide: 400}}
                           overlay={<Tooltip id="button-tooltip">{text}</Tooltip>}
    >{obj}</OverlayTrigger>
}


export function isNumber(n) {
    return !isNaN(parseFloat(n)) && !isNaN(n - 0)
}

export function isHex(h) {
    if (isNumber(h)) {
        return false
    }
    let hexRe = /[0-9A-Fa-f]{6}/g;
    return hexRe.test(h)
}

export function upperCaseAndRepUnderscore(str) {
    let i, frags = str.split('_');
    for (i = 0; i < frags.length; i++) {
        frags[i] = frags[i].charAt(0).toUpperCase() + frags[i].slice(1);
    }
    return frags.join(' ');
}

export function convertTransaction(v) {
    let value = Object.assign({}, v)
    delete value.transaction
    return convertNonIndexTransaction(value)
}

export function cpyObj(v) {
    return Object.assign({}, v)
}

export function pagination(data, callback) {
    let pageSquares = []
    if ("perPage" in data) {
        let start = data.pageNumber - 2
        if (start <= 0) {
            start = 1
        }
        for (let i = start; i <= Math.min(Math.ceil(data.totalPages), start + 5); i++) {
            pageSquares.push(
                <Pagination.Item key={i} onClick={() => callback(i)} active={i === data.pageNumber}>
                    {i}
                </Pagination.Item>,
            );
        }
    }
    return <Pagination className="pagination">{pageSquares}<Pagination.Ellipsis/></Pagination>
}

export function isEmpty(obj) {
    return Object.keys(obj).length === 0
}

export function clipboard(state, setState, detail) {
    navigator.clipboard.writeText(detail)
    setState({...state, showToast: true})
}

export function convertNonIndexTransaction(tx) {
    if (tx.recipient == null) {
        tx.recipient = tx.sender
    }
    if (!("index" in tx) || tx.index === 0) {
        tx.index = 0
    }
    tx = JSON.parse(JSON.stringify(tx, ["sender", "recipient", "message_type", "height", "index", "tx_hash", "fee", "sequence"], 4))
    return tx
}

export function convertNonIndexTransactions(txs) {
    for (let i = 0; i < txs.length; i++) {
        txs[i] = convertNonIndexTransaction(txs[i])
    }
    return txs
}

export function formatBlock(blk) {
    // list of values to omit
    let {
        last_quorum_certificate, next_validator_root, state_root, transaction_root,
        validator_root, last_block_hash, network_id, vdf, ...value
    } = blk.block.block_header
    return value
}

export function formatCertificateResults(qc) {
    return {
        "certificate_height": qc.header.height,
        "network_id": qc.header.networkID,
        "committee_id": qc.header.committeeID,
        "block_hash": qc.blockHash,
        "results_hash": qc.resultsHash,
    }
}