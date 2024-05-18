import {useState} from 'react';
import {
    Pagination,
    Modal,
    Table,
    Tooltip,
    OverlayTrigger,
    Tab,
    Tabs,
    CardGroup,
    Card, Toast, ToastContainer
} from 'react-bootstrap'
import Truncate from 'react-truncate-inside';
import {JsonViewer} from '@textea/json-viewer'

async function setModalData(setModalState, state, getModalData, page) {
    let newData = await getModalData(state.query, page)
    setModalState({show: true, query: state.query, page: active, data: newData})
}

let active = 1;

function getPagination(data, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber) {
    const pageSquares = [];
    let total = 0
    let callback = (number) => setBlockPage(number)
    if ("block" in data) {
        active = blockPage
        total = Math.ceil(data.block.block_header.num_txs / 10)
    } else if ("account" in data) {
        if (tabNumber === 2) {
            active = data.sent_transactions.pageNumber
            total = data.sent_transactions.totalPages
            callback = (number) => setModalData(setModalState, state, getModalData, number)
        } else {
            active = data.rec_transactions.pageNumber
            total = data.rec_transactions.totalPages
            callback = (number) => setModalData(setModalState, state, getModalData, number)
        }
    }
    let startPage = active - 2
    if (startPage <= 0) {
        startPage = 1
    }
    for (let number = startPage; number <= Math.min(Math.ceil(total), startPage + 5); number++) {
        pageSquares.push(
            <Pagination.Item key={number} onClick={() => callback(number)} active={number === active}>
                {number}
            </Pagination.Item>,
        );
    }
    return pageSquares
}

function getTitle(v) {
    let value = Object.assign({}, v)
    if ("transaction" in value) {
        return "Transaction"
    } else if ("block" in value) {
        return "Block"
    } else if ("account" in value) {
        return "Account"
    } else if ("validator" in value) {
        return "Validator"
    }
    return ""
}

function filterHighlights(v) {
    if (v === null || v === undefined) {
        return {"None": ""}
    }
    let value = Object.assign({}, v)
    if ("transaction" in value) {
        delete value.transaction
        return value
    } else if ("block" in value) {
        value = Object.assign({}, v.block.block_header)
        delete value.last_quorum_certificate
        delete value.next_validator_root
        delete value.state_root
        delete value.transaction_root
        delete value.validator_root
        delete value.last_block_hash
        delete value.network_id
        return value
    } else if ("account" in value) {
        return value.account
    } else if ("validator" in value) {
        return value.validator
    }
}

function filterPaginatedValues(v) {
    if (v == null || v === 0) {
        return [0]
    }
    if ("block" in v) {
        let value = Object.assign({}, v)
        if (value === null || value === undefined) {
            return {"None": ""}
        }
        delete value.last_quorum_certificate
        delete value.took
        delete value.size
        delete value.transaction
        return value
    } else if ("transaction" in v) {
        let value = Object.assign({}, v)
        delete (value.transaction)
        return value
    }
    return v
}

function copy(detail) {
    navigator.clipboard.writeText(detail)
}

function formatTitle(str) {
    let i, frags = str.split('_');
    for (i = 0; i < frags.length; i++) {
        frags[i] = frags[i].charAt(0).toUpperCase() + frags[i].slice(1);
    }
    return frags.join(' ');
}

function formatTime(key, value) {
    if (key.includes("time")) {
        let d = new Date(Date.parse(value + " UTC"))
        return (d.toDateString() + " " + d.toTimeString())
    } else {
        return value
    }
}

function getTabTitle(v, tabNumber) {
    if ("transaction" in v) {
        if (tabNumber === 1) {
            return "Message"
        } else if (tabNumber === 2) {
            return "Meta"
        } else {
            return "Raw"
        }
    } else if ("block" in v) {
        if (tabNumber === 1) {
            return "Header"
        } else if (tabNumber === 2) {
            return "Transactions"
        } else {
            return "Raw"
        }
    } else if ("account" in v) {
        if (tabNumber === 1) {
            return "Account"
        } else if (tabNumber === 2) {
            return "Sent Transactions"
        } else {
            return "Received Transactions"
        }
    } else if ("validator" in v) {
        if (tabNumber === 1) {
            return "Validator"
        } else if (tabNumber === 2) {
            return "Stats"
        } else {
            return "Raw"
        }
    }
}

function getTabContent(v, tabNumber) {
    if ("block" in v) {
        if (tabNumber === 1) {
            let value = Object.assign({}, v.block.block_header)
            delete value.last_quorum_certificate
            delete value.took
            delete value.size
            return value
        } else if (tabNumber === 2) {
            if (v.block.transactions == null) {
                return 0
            }
            v.block.transactions[0].index = 0
            v.block.transactions[0] =
                JSON.parse(JSON.stringify(v.block.transactions[0], ["sender", "recipient", "message_type", "height", "index", "tx_hash"], 4))
            return v.block.transactions
        } else {
            return v.block
        }
    } else if ("transaction" in v) {
        if (tabNumber === 1) {
            return Object.assign({}, v.transaction.msg)
        } else if (tabNumber === 2) {
            let value = Object.assign({}, v.transaction)
            delete value.msg
            delete value.signature
            return value
        } else {
            return v
        }
    } else if ("account" in v) {
        if (tabNumber === 1) {
            return Object.assign({}, v.account)
        } else if (tabNumber === 2) {
            return convertNonIndexTransactions(v.sent_transactions)
        } else {
            return convertNonIndexTransactions(v.rec_transactions)
        }
    } else if ("validator" in v) {
        return Object.assign({}, v.validator)
    }
}

function convertNonIndexTransactions(txs) {
    for (let i = 0; i < txs.results.length; i++) {
        if ("index" in txs.results[i] && txs.results[i].index !== 0) {
            continue
        }
        txs.results[i].index = 0
        txs.results[i] = JSON.parse(JSON.stringify(txs.results[i], ["sender", "recipient", "message_type", "height", "index", "tx_hash"], 4))
    }
    return txs.results
}

function getPaginatedContent(v, tabNumber) {
    if ("block" in v) {
        if (v.block.transactions == null) {
            return [0]
        }
        return v.block.transactions
    } else if ("account" in v) {
        if (tabNumber === 2) {
            return convertNonIndexTransactions(v.sent_transactions)
        } else {
            return convertNonIndexTransactions(v.rec_transactions)
        }
    }
}

function renderTableBody(data, tabNumber) {
    return <Table responsive>
        <tbody>
        {Object.keys(getTabContent(data, tabNumber)).map((k, i) => {
            let focusedData = getTabContent(data, tabNumber)
            return <tr key={i}>
                <td className="detail-table-title">{formatTitle(k)}</td>
                <td className="detail-table-info">{formatTime(k, focusedData[k])}</td>
            </tr>
        })}
        </tbody>
    </Table>
}

function getBlockPageStart(bp) {
    if (bp === 0) {
        return 0
    }
    return bp * 10 - 10
}


function getBlockPageEnd(bp) {
    if (bp === 0) {
        return 10
    }
    return bp * 10
}

function renderTableBodyPaginated(data, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber) {
    return <div><Table responsive>
        <tbody>
        <tr>
            {Object.keys(filterPaginatedValues(getTabContent(data, 2)[0])).map((k, i) => (
                <td key={i} className="detail-table-row-title">{formatTitle(k)}</td>
            ))
            }
        </tr>
        {getPaginatedContent(data, tabNumber).slice(getBlockPageStart(blockPage), getBlockPageEnd(blockPage)).map((item, idx) => (
                <tr key={idx}>
                    {Object.keys(filterPaginatedValues(item)).map((k, i) => (
                        <td key={i} className="detail-table-row-info">
                            <span>{formatTime(k, item[k])}</span>
                        </td>
                    ))}
                </tr>
            )
        )}
        </tbody>
    </Table>
        <Pagination
            style={{float: 'right'}}>{getPagination(data, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber)}
            <Pagination.Ellipsis/></Pagination>
    </div>
}

function renderJSONRaw(data) {
    return <JsonViewer rootName={getTitle(data)} defaultInspectDepth={1} value={getTabContent(data, 3)}/>
}

function selectTabContent(v, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber) {
    if ("block" in v) {
        if (tabNumber === 1) {
            return renderTableBody(v, tabNumber)
        } else if (tabNumber === 2) {
            return renderTableBodyPaginated(v, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber)
        } else {
            return renderJSONRaw(v)
        }
    } else if ("transaction" in v) {
        if (tabNumber === 1) {
            return renderTableBody(v, tabNumber)
        } else if (tabNumber === 2) {
            return renderTableBody(v, tabNumber)
        } else {
            return renderJSONRaw(v)
        }
    } else if ("account" in v) {
        if (tabNumber === 1) {
            return renderTableBody(v, tabNumber)
        } else if (tabNumber === 2) {
            return renderTableBodyPaginated(v, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber)
        } else {
            return renderTableBodyPaginated(v, blockPage, setBlockPage, getModalData, state, setModalState, tabNumber)
        }
    } else if ("validator" in v) {
        if (tabNumber === 1) {
            return renderTableBody(v, tabNumber)
        } else if (tabNumber === 2) {
            return <p>coming soon</p>
        } else {
            return renderJSONRaw(v)
        }
    }
}

function isEmpty(obj) {
    for (const prop in obj) {
        if (Object.hasOwn(obj, prop)) {
            return false;
        }
    }

    return true;
}

function DetailModal({blockPage, setBlockPage, getModalData, state, setModalState, handleClose}) {
    const data = state.data
    if (isEmpty(data)) {
        return <></>
    }
    if (data === "") {
        return <></>
    }
    if (data === "no result found") {
        return <ToastContainer position={"top-center"}
                               style={{position: "fixed", marginTop: "15px", marginLeft: "50px"}}>
            <Toast onClose={() => {
                setModalState({show: false, page: 0, data: ""})
            }} show={true} delay={3000} autohide>
                <Toast.Header>
                    <strong className="me-auto">Warning</strong>
                    <small>from search</small>
                </Toast.Header>
                <Toast.Body style={{fontWeight: "bold", color: "darkred"}}>no results found</Toast.Body>
            </Toast></ToastContainer>
    }
    let highlights = filterHighlights(data)
    return (
        <>
            <Modal size="xl" show={state.show} onHide={handleClose}>
                <Modal.Header closeButton>
                </Modal.Header>
                <Modal.Body className="modal-body">
                    <h3 className="modal-header">
                        <div className="modal-header-icon"/>
                        {getTitle(data)} Details
                    </h3>
                    <CardGroup style={{margin: "20px 0px 20px 0px"}}>
                        {Object.keys(highlights).map((k, i) => (
                            <OverlayTrigger
                                key={i}
                                placement="top"
                                delay={{show: 200, hide: 200}}
                                overlay={<Tooltip id="button-tooltip">{highlights[k]}</Tooltip>}
                            >
                                <Card onClick={() => {
                                    copy(highlights[k])
                                }}
                                      key={i} className="modal-card-container">
                                    <Card.Body className="modal-card">
                                        <h5 className="modal-card-title">{k}</h5>
                                        <div className="modal-card-detail"><Truncate text={highlights[k]}/></div>
                                        <img src="./copy.png" style={{
                                            height: "10px",
                                            width: "10px",
                                            float: "right",
                                            position: "relative",
                                            right: "5px",
                                            bottom: "10px"
                                        }}/>
                                    </Card.Body>
                                </Card>
                            </OverlayTrigger>
                        ))}
                    </CardGroup>
                    <Tabs
                        defaultActiveKey="default"
                        id="fill-tab-example"
                        className="mb-3"
                        fill
                    >
                        <Tab tabClassName="rb-tab" eventKey="default" title={getTabTitle(data, 1)}>
                            {selectTabContent(data, blockPage, setBlockPage, getModalData, state, setModalState, 1)}
                        </Tab>
                        <Tab tabClassName="rb-tab" eventKey="tab2" title={getTabTitle(data, 2)}>
                            {selectTabContent(data, blockPage, setBlockPage, getModalData, state, setModalState, 2)}
                        </Tab>
                        <Tab tabClassName="rb-tab" eventKey="tab3" title={getTabTitle(data, 3)}>
                            {selectTabContent(data, blockPage, setBlockPage, getModalData, state, setModalState, 3)}
                        </Tab>
                    </Tabs>
                </Modal.Body>
            </Modal>
        </>
    );
}

export default DetailModal;