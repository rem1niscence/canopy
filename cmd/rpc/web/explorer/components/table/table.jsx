import {Table} from "react-bootstrap";
import Truncate from 'react-truncate-inside';
import {
    convertTransaction, cpyObj, formatIfTime, formatNumberWCommas,
    isHex, isNumber, pagination, upperCaseAndRepUnderscore
} from "@/components/util";

function formatValue(k, v, openModal) {
    if (k === "public_key") {
        return <Truncate text={v}/>
    }
    if (isHex(v) || k === "height") {
        if (isNumber(v)) {
            return <a href={"#"} onClick={() => openModal(v)} style={{cursor: "pointer"}}>
                {v}
            </a>
        }
        return <a href={"#"} onClick={() => openModal(v)} style={{cursor: "pointer"}}>
            <Truncate text={v}/>
        </a>
    }
    if (isNumber(v)) {
        return formatNumberWCommas(v)
    }
    return formatIfTime(k, v)
}

function convertBlock(v) {
    let {
        last_quorum_certificate, next_validator_root, state_root, transaction_root,
        validator_root, last_block_hash, network_id, vdf, ...value
    } = cpyObj(v.block_header)
    if ("num_txs" in v.block_header) {
        return value
    } else {
        value.num_txs = "0"
        return JSON.parse(JSON.stringify(value, ["height", "hash", "time", "num_txs", "total_txs", "proposer_address"], 4))
    }
}

function convertParams(v) {
    if (v.Consensus == null) {
        return ["0"]
    }
    let result = [], value = cpyObj(v)
    Object.keys(value.Consensus).map(function (k) {
        result.push({"ParamName": k, "ParamValue": value["Consensus"][k], "ParamSpace": "Consensus"})
    })
    Object.keys(value.Validator).map(function (k) {
        result.push({"ParamName": k, "ParamValue": value["Validator"][k], "ParamSpace": "Validator"})
    })
    Object.keys(value.Fee).map(function (k) {
        result.push({"ParamName": k, "ParamValue": value["Fee"][k], "ParamSpace": "Fee"})
    })
    Object.keys(value.Governance).map(function (k) {
        result.push({"ParamName": k, "ParamValue": value["Governance"][k], "ParamSpace": "Governance"})
    })
    return result
}

function getHeader(v) {
    switch (v.type) {
        case "tx-results-page":
            return "Transactions"
        case "pending-results-page":
            return "Pending"
        case "block-results-page":
            return "Blocks"
        case "accounts":
            return "Accounts"
        case "validators":
            return "Validators"
        default:
            return "Governance"
    }
}

function getTableHeader(v) {
    switch (v.type) {
        case "tx-results-page":
        case "pending-results-page":
            if (v.results == null) {
                return [0]
            }
            return convertTransaction(v.results[0])
        case "block-results-page":
            console.log(v.results)
            return convertBlock(v.results[0])
        case "accounts":
            return v.results[0]
        case "validators":
            return v.results[0]
        default:
            return {"Name": "", "Value": "", "Space": ""}
    }
}

function getTableBody(v) {
    let result = []
    switch (v.type) {
        case "tx-results-page":
        case "pending-results-page":
            if (v.results == null) {
                return [0]
            }
            v.results.forEach(function (item) {
                result.push(convertTransaction(item))
            })
            break
        case "block-results-page":
            v.results.forEach(function (item) {
                result.push(convertBlock(item))
            })
            break
        case "accounts":
            v.results.forEach(function (item) {
                result.push(item)
            })
            break
        case "validators":
            v.results.forEach(function (item) {
                result.push(item)
            })
            break
        default:
            return convertParams(v)
    }
    return result
}


function DataTable(props) {
    return <>
        <div className="data-table">
            <div className="data-table-content">
                <h5 className="data-table-head">{getHeader(props.state.tableData)}</h5>
            </div>
            <Table responsive bordered hover size="sm" className="table">
                <thead>
                <tr>
                    {Object.keys(getTableHeader(props.state.tableData)).map((s, i) => (
                        <th key={i} className="table-head">{upperCaseAndRepUnderscore(s)}</th>
                    ))}
                </tr>
                </thead>
                <tbody>
                {getTableBody(props.state.tableData).map((val, idx) => (
                    <tr key={idx}>
                        {Object.keys(val).map((k, i) => {
                            return <td key={i} className="table-col">
                                {formatValue(k, val[k], props.openModal)}
                            </td>
                        })}
                    </tr>
                ))}
                </tbody>
            </Table>
            {pagination(props.state.tableData, (i) => props.selectTable(props.state.category, i))}
        </div>
    </>
}

export default DataTable;