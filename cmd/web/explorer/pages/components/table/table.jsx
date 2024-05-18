import Navbar from "react-bootstrap/Navbar";
import Container from "react-bootstrap/Container";
import {Form, Pagination, Table} from "react-bootstrap";
import Truncate from 'react-truncate-inside';

function isNumber(n) {
    return !isNaN(parseFloat(n)) && !isNaN(n - 0)
}

function isHex(h) {
    if (isNumber(h)) {
        return false
    }
    let hexRe = /[0-9A-Fa-f]{6}/g;
    return hexRe.test(h)
}

function formatHeader(str) {
    let i, frags = str.split('_');
    for (i = 0; i < frags.length; i++) {
        frags[i] = frags[i].charAt(0).toUpperCase() + frags[i].slice(1);
    }
    return frags.join(' ');
}

function formatTime(key, value) {
    if (key.includes("time")) {
        let d = new Date(Date.parse(value + " UTC"))
        return (d.toLocaleString())
    } else {
        return value
    }
}

function formatValue(k, v, showModal) {
    if (isHex(v)) {
        if (k === "public_key") {
            return <Truncate text={v}/>
        }
        return <a href={"#"} onClick={() => showModal(v, 0)} style={{cursor: "pointer"}}>
            <Truncate text={v}/>
        </a>
    } else if (k === "height") {
        return <a href={"#"} onClick={() => showModal(v, 0)} style={{cursor: "pointer"}}>
            {v}
        </a>
    }
    if (isNumber(v)) {
        return numberWithCommas(v)
        // return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 4}).format(v)
    }
    return formatTime(k, v)
}

function numberWithCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function convertTransaction(v) {
    let value = Object.assign({}, v)
    if (value.message_type === "send") {
        value.amount = value.transaction.msg.amount
        value.fee = value.transaction.fee
        value.sequence = value.transaction.sequence
    }
    delete value.transaction
    return convertNonIndexTransactions(value)
}

function convertNonIndexTransactions(tx) {
    if ("index" in tx && tx.index !== 0) {
        return tx
    }
    tx.index = 0
    tx = JSON.parse(JSON.stringify(tx, ["sender", "recipient", "message_type", "height", "index", "tx_hash", "amount", "fee", "sequence"], 4))
    return tx
}

function convertBlock(v) {
    let value = Object.assign({}, v.block_header)
    delete value.last_quorum_certificate
    delete value.next_validator_root
    delete value.state_root
    delete value.transaction_root
    delete value.validator_root
    delete value.last_block_hash
    delete value.network_id
    if ("num_txs" in v.block_header) {
        return value
    } else {
        value.num_txs = "0"
        return JSON.parse(JSON.stringify(value, ["height", "hash", "time", "num_txs", "total_txs", "proposer_address"], 4))
    }
}

function convertParams(v) {
    let result = []
    let value = Object.assign({}, v)
    Object.keys(value.Consensus).map(function (k, index) {
        result.push({
            "ParamName": k,
            "ParamValue": value.Consensus[k],
            "ParamSpace": "Consensus"
        })
    })
    Object.keys(value.Validator).map(function (k, index) {
        result.push({
            "ParamName": k,
            "ParamValue": value.Validator[k],
            "ParamSpace": "Validator"
        })
    })
    Object.keys(value.Fee).map(function (k, index) {
        result.push({
            "ParamName": k,
            "ParamValue": value.Fee[k],
            "ParamSpace": "Fee"
        })
    })
    Object.keys(value.Governance).map(function (k, index) {
        result.push({
            "ParamName": k,
            "ParamValue": value.Governance[k],
            "ParamSpace": "Governance"
        })
    })
    return result
}

let active = 1;

function getPagination(data, index, selectTable) {
    if ("perPage" in data) {
        const pageSquares = [];
        active = data.pageNumber
        let startPage = active - 2
        if (startPage <= 0) {
            startPage = 1
        }
        for (let number = startPage; number <= Math.min(Math.ceil(data.totalPages), startPage + 5); number++) {
            pageSquares.push(
                <Pagination.Item key={number} onClick={() => selectTable(index, number)} active={number === active}>
                    {number}
                </Pagination.Item>,
            );
        }
        return pageSquares
    } else {
        return []
    }
}

function getHeader(v) {
    if ("type" in v) {
        if (v.type === "tx-results-page") {
            return "Transactions"
        } else if (v.type === "pending-results-page") {
            return "Pending"
        } else if ((v.type === "block-results-page")) {
            return "Blocks"
        } else if ((v.type === "accounts")) {
            return "Accounts"
        } else if ((v.type === "validators")) {
            return "Validators"
        }
    } else {
        return "Governance"
    }
}

function getTableHeader(v) {
    if ("type" in v) {
        if (v == null || v.results == null || v.results.length === 0) {
            return ["data"]
        }
        if (v.type === "tx-results-page" || v.type === "pending-results-page") {
            return convertTransaction(v.results[0])
        } else if (v.type === "block-results-page") {
            return convertBlock(v.results[0])
        } else if (v.type === "accounts") {
            return v.results[0]
        } else if (v.type === "validators") {
            return v.results[0]
        }
    } else {
        return {"Name": "", "Value": "", "Space": ""}
    }
}

function getTableBody(v) {
    if ("type" in v) {
        if (v == null || v.results == null || v.results.length === 0) {
            return [0]
        }
        let result = []
        if (v.type === "tx-results-page" || v.type === "pending-results-page") {
            v.results.forEach(function (item) {
                result.push(convertTransaction(item))
            });
        } else if (v.type === "block-results-page") {
            v.results.forEach(function (item) {
                result.push(convertBlock(item))
            });
        } else if (v.type === "accounts") {
            v.results.forEach(function (item) {
                result.push(item)
            });
        } else if (v.type === "validators") {
            v.results.forEach(function (item) {
                result.push(item)
            });
        }
        return result
    }
    return convertParams(v)
}


function DataTable(props) {
    return (<div style={{overflow: "scroll"}}>
        <div style={{margin: "20px 20px 20px 10px"}}>
            <h5 style={{fontWeight: "bold"}}>{getHeader(props.newData)}</h5>
        </div>
        <Table responsive bordered hover size="sm" className="table">
            <thead>
            <tr style={{fontSize: "12px"}}>
                {Object.keys(getTableHeader(props.newData)).map((s, i) => (
                    <th key={i}
                        style={{paddingLeft: "10px", paddingTop: "15px", paddingBottom: "15px"}}>{formatHeader(s)}</th>
                ))}
            </tr>
            </thead>
            <tbody>
            {getTableBody(props.newData).map((val, idx) => (
                <tr key={idx}>
                    {Object.keys(val).map((k, i) => {
                        return <td key={i} style={{
                            maxWidth: "200px",
                            fontSize: "13px",
                            paddingLeft: "10px",
                            paddingTop: "15px",
                            paddingBottom: "15px"
                        }}>
                            {formatValue(k, val[k], props.handleOpen)}
                        </td>
                    })}
                </tr>
            ))}
            </tbody>
        </Table>
        <Pagination style={{float: 'right'}}>{getPagination(props.newData, props.index, props.selectTable)}
            <Pagination.Ellipsis/></Pagination>
    </div>);
}

export default DataTable;