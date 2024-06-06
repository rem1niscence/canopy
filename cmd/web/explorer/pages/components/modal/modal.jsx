import React from "react";
import Truncate from 'react-truncate-inside';
import {JsonViewer} from '@textea/json-viewer'
import {Modal, Table, Tab, Tabs, CardGroup, Card, Toast, ToastContainer, Button} from 'react-bootstrap'
import * as API from "@/pages/components/api";
import {
    clipboard, convertNonIndexTransactions, cpyObj, formatBlock, formatIfTime, isEmpty,
    pagination, upperCaseAndRepUnderscore, withTooltip
} from "@/pages/components/util";

function formatCardData(state, v) {
    if (v === null || v === undefined) {
        return {"None": ""}
    }
    let value = cpyObj(v)
    if ("transaction" in value) {
        delete value.transaction
        return value
    } else if ("block" in value) {
        return formatBlock(value)
    } else if ("validator" in value && !state.modalState.accOnly) {
        return value.validator
    } else if ("account" in value) {
        return value.account
    }
}

function formatPaginated(v) {
    if (v == null || v === 0) {
        return [0]
    }
    if ("block" in v) {
        return v == null ? {"None": ""} : formatBlock(v)
    } else if ("transaction" in v) {
        let {transaction, ...value} = v
        return value
    }
    return v
}

function formatTabData(state, v, tab) {
    if ("block" in v) {
        if (tab === 0) {
            return formatBlock(v)
        } else if (tab === 1) {
            return v.block.transactions == null ? 0 : convertNonIndexTransactions(v.block.transactions)
        } else {
            return v.block
        }
    } else if ("transaction" in v) {
        if (tab === 0) {
            return cpyObj(v.transaction.msg)
        } else if (tab === 1) {
            let {msg, signature, ...value} = v.transaction
            return value
        } else {
            return v
        }
    } else if ("validator" in v && !state.modalState.accOnly) {
        return cpyObj(v.validator)
    } else if ("account" in v) {
        if (tab === 0) {
            return cpyObj(v.account)
        } else if (tab === 1) {
            return convertNonIndexTransactions(v.sent_transactions.results)
        } else {
            return convertNonIndexTransactions(v.rec_transactions.results)
        }
    }
}

function getModalTitle(state, v) {
    let value = cpyObj(v)
    if ("transaction" in value) {
        return "Transaction"
    } else if ("block" in value) {
        return "Block"
    } else if ("validator" in value && !state.modalState.accOnly) {
        return "Validator"
    } else if ("account" in value) {
        return "Account"
    }
    return ""
}

function getTabTitle(state, data, tab) {
    if ("transaction" in data) {
        if (tab === 0) {
            return "Message"
        } else if (tab === 1) {
            return "Meta"
        } else {
            return "Raw"
        }
    } else if ("block" in data) {
        if (tab === 0) {
            return "Header"
        } else if (tab === 1) {
            return "Transactions"
        } else {
            return "Raw"
        }
    } else if ("validator" in data && !state.modalState.accOnly) {
        if (tab === 0) {
            return "Validator"
        } else if (tab === 1) {
            return "Account"
        } else {
            return "Raw"
        }
    } else if ("account" in data) {
        if (tab === 0) {
            return "Account"
        } else if (tab === 1) {
            return "Sent Transactions"
        } else {
            return "Received Transactions"
        }
    }
}

export default function DetailModal({state, setState}) {
    const data = state.modalState.data, cards = formatCardData(state, data)

    if (isEmpty(data)) {
        return <></>
    } else if (data === "no result found") {
        return <>
            <ToastContainer position={"top-center"} className="search-toast">
                <Toast onClose={resetState} show={true} delay={3000} autohide>
                    <Toast.Header/>
                    <Toast.Body className="search-toast-body">no results found</Toast.Body>
                </Toast>
            </ToastContainer>
        </>
    }

    function resetState() {
        setState({...state, modalState: {show: false, query: "", page: 0, data: {}, accOnly: false}});
    }

    // TABS RENDERINGS BELOW
    function renderTab(tab) {
        if ("block" in data) {
            if (tab === 0) {
                return renderBasicTable(tab)
            } else if (tab === 1) {
                return renderPageTable(tab)
            } else {
                return renderJSONViewer()
            }
        } else if ("transaction" in data) {
            if (tab === 0) {
                return renderBasicTable(tab)
            } else if (tab === 1) {
                return renderBasicTable(tab)
            } else {
                return renderJSONViewer()
            }
        } else if ("validator" in data && !state.modalState.accOnly) {
            if (tab === 0) {
                return renderBasicTable(tab)
            } else if (tab === 1) {
                return renderTableButton()
            } else {
                return renderJSONViewer()
            }
        } else if ("account" in data) {
            if (tab === 0) {
                return renderBasicTable(tab)
            } else {
                return renderPageTable(tab)
            }
        }
    }

    function renderBasicTable(tab) {
        let body = formatTabData(state, data, tab)
        return <>
            <Table responsive>
                <tbody>
                {Object.keys(body).map((k, i) => {
                    return <tr key={i}>
                        <td className="detail-table-title">{upperCaseAndRepUnderscore(k)}</td>
                        <td className="detail-table-info">{formatIfTime(k, body[k])}</td>
                    </tr>
                })}
                </tbody>
            </Table>
        </>
    }

    function renderPageTable(tab) {
        let start = 0, end = 10, page = [0], d = data, ms = state.modalState, blk = d.block
        if ("block" in d) {
            end = [0, 1].includes(ms.page) ? 10 : ms.page * 10
            start = end - 10
            page = blk.transactions == null ? page : blk.transactions
            d = {pageNumber: ms.Page, perPage: 10, totalPages: Math.ceil(blk.block_header.num_txs / 10), ...d}
        } else if ("account" in d) {
            if (tab === 1) {
                page = convertNonIndexTransactions(d.sent_transactions.results)
                d = d.sent_transactions
            } else {
                page = convertNonIndexTransactions(d.rec_transactions.results)
                d = d.rec_transactions
            }
        }

        return <>
            <Table responsive>
                <tbody>
                <tr>
                    {Object.keys(formatPaginated(formatTabData(state, data, 1)[0])).map((k, i) => {
                        return <td key={i} className="detail-table-row-title">{upperCaseAndRepUnderscore(k)}</td>
                    })}
                </tr>
                {page.slice(start, end).map((item, key) => {
                        return <tr key={key}>
                            {Object.keys(formatPaginated(item)).map((k, i) => (
                                <td key={i} className="detail-table-row-info">
                                    {formatIfTime(k, item[k])}
                                </td>
                            ))}
                        </tr>
                    }
                )}
                </tbody>
            </Table>
            {pagination(d, (p) => API.getModalData(ms.query, p).then(r => {
                setState({...state, modalState: {...ms, show: true, query: ms.query, page: p, data: r}})
            }))}
        </>
    }

    function renderJSONViewer() {
        return <JsonViewer rootName={"result"} defaultInspectDepth={1} value={formatTabData(state, data, 2)}/>
    }

    function renderTableButton() {
        return <>
            <Button className="open-acc-details-btn" variant="outline-secondary"
                    onClick={() => setState({...state, modalState: {...state.modalState, accOnly: true}})}>
                Open Account Details
            </Button>
        </>
    }

    return <>
        <Modal size="xl" show={state.modalState.show} onHide={resetState}>
            <Modal.Header closeButton/>
            <Modal.Body className="modal-body">
                {/* TITLE */}
                <h3 className="modal-header">
                    <div className="modal-header-icon"/>
                    {getModalTitle(state, data)} Details
                </h3>
                {/* CARDS */}
                <CardGroup className="modal-card-group">
                    {Object.keys(cards).map((k, i) => {
                        return withTooltip(
                            <Card onClick={() => clipboard(state, setState, cards[k])} key={i} className="modal-cards">
                                <Card.Body className="modal-card">
                                    <h5 className="modal-card-title">{k}</h5>
                                    <div className="modal-card-detail">
                                        <Truncate text={String(cards[k])}/>
                                    </div>
                                    <img className="copy-img" src="./copy.png"/>
                                </Card.Body>
                            </Card>, cards[k], i, "top")
                    })}
                </CardGroup>
                {/* TABS */}
                <Tabs defaultActiveKey="0" id="fill-tab-example" className="mb-3" fill>
                    {[...Array(3)].map((_, i) => {
                        return <Tab key={i} tabClassName="rb-tab" eventKey={i} title={getTabTitle(state, data, i)}>
                            {renderTab(i)}
                        </Tab>
                    })}
                </Tabs>
            </Modal.Body>
        </Modal>
    </>
}