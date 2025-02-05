import React from "react";
import Truncate from "react-truncate-inside";
import { JsonViewer } from "@textea/json-viewer";
import { Modal, Table, Tab, Tabs, CardGroup, Card, Toast, ToastContainer, Button } from "react-bootstrap";
import * as API from "@/components/api";
import {
  clipboard,
  cpyObj,
  convertIfTime,
  isEmpty,
  pagination,
  upperCaseAndRepUnderscore,
  withTooltip,
  convertTx,
  toCNPY,
} from "@/components/util";

// convertCardData() converts the data from state into a display object for rendering
function convertCardData(state, v) {
  if (!v) return { None: "" };
  const value = cpyObj(v);
  if (value.transaction) {
    delete value.transaction;
    return value;
  }
  return value.block
    ? convertBlock(value)
    : value.validator && !state.modalState.accOnly
      ? value.validator
      : value.account;
}

// convertPaginated() converts a paginated item into a display object for rendering
function convertPaginated(v) {
  if (v == null || v === 0) return [0];
  if ("block" in v) return convertBlock(v) || { None: "" };
  if ("transaction" in v) return { ...v, transaction: undefined };
  return v;
}

// convertTransactions() converts an array of transactions into a suitable display object
export function convertTransactions(txs) {
  for (let i = 0; i < txs.length; i++) {
    txs[i] = convertTx(txs[i]);
  }
  return txs;
}

// convertBlock() converts a block item into a display object for rendering
export function convertBlock(blk) {
  let {
    last_quorum_certificate,
    next_validator_root,
    state_root,
    transaction_root,
    validator_root,
    last_block_hash,
    network_id,
    vdf,
    ...value
  } = blk.block.block_header;
  return value;
}

// convertCertificateResults() converts a qc item into a display object for rendering
export function convertCertificateResults(qc) {
  return {
    certificate_height: qc.header.height,
    network_id: qc.header.networkID,
    committee_id: qc.header.committeeID,
    block_hash: qc.blockHash,
    results_hash: qc.resultsHash,
  };
}

// convertTabData() converts the modal data into specific tab display object for rendering
function convertTabData(state, v, tab) {
  if ("block" in v) {
    switch (tab) {
      case 0:
        return convertBlock(v);
      case 1:
        return v.block.transactions ? convertTransactions(v.block.transactions) : 0;
      default:
        return v.block;
    }
  } else if ("transaction" in v) {
    switch (tab) {
      case 0:
        if ("qc" in v.transaction.msg) return convertCertificateResults(v.transaction.msg.qc);
        return v.transaction.msg;
      case 1:
        return { hash: v.tx_hash, time: v.transaction.time, sender: v.sender, type: v.message_type };
      default:
        return v;
    }
  } else if ("validator" in v && !state.modalState.accOnly) {
    return cpyObj(v.validator);
  } else if ("account" in v) {
    switch (tab) {
      case 0:
        let account = cpyObj(v.account);
        account.amount = toCNPY(account.amount);
        return account;
      case 1:
        return convertTransactions(v.sent_transactions.results);
      default:
        return convertTransactions(v.rec_transactions.results);
    }
  }
}

// getModalTitle() extracts the modal title from the object
function getModalTitle(state, v) {
  if ("transaction" in v) return "Transaction";
  if ("block" in v) return "Block";
  if ("validator" in v && !state.modalState.accOnly) return "Validator";
  return "Account";
}

// getTabTitle() extracts the tab title from the object
function getTabTitle(state, data, tab) {
  if ("transaction" in data) {
    return tab === 0 ? "Message" : tab === 1 ? "Meta" : "Raw";
  }
  if ("block" in data) {
    return tab === 0 ? "Header" : tab === 1 ? "Transactions" : "Raw";
  }
  if ("validator" in data && !state.modalState.accOnly) {
    return tab === 0 ? "Validator" : tab === 1 ? "Account" : "Raw";
  }
  return tab === 0 ? "Account" : tab === 1 ? "Sent Transactions" : "Received Transactions";
}

// DetailModal() returns the main modal component for this file
export default function DetailModal({ state, setState }) {
  const data = state.modalState.data;
  const cards = convertCardData(state, data);

  // check if the data is empty or no results
  if (isEmpty(data)) return <></>;

  if (data === "no result found") {
    return (
      <ToastContainer position={"top-center"} className="search-toast">
        <Toast onClose={resetState} show delay={3000} autohide>
          <Toast.Header />
          <Toast.Body className="search-toast-body">no results found</Toast.Body>
        </Toast>
      </ToastContainer>
    );
  }

  // resetState() resets the modal state back to initial
  function resetState() {
    setState({ ...state, modalState: { show: false, query: "", page: 0, data: {}, accOnly: false } });
  }

  // renderTab() renders a tab based on the state data and tab number
  function renderTab(tab) {
    if ("block" in data) {
      return tab === 0 ? renderBasicTable(tab) : tab === 1 ? renderPageTable(tab) : renderJSONViewer();
    }
    if ("transaction" in data) {
      return tab === 0 ? renderBasicTable(tab) : tab === 1 ? renderBasicTable(tab) : renderJSONViewer();
    }
    if ("validator" in data && !state.modalState.accOnly) {
      return tab === 0 ? renderBasicTable(tab) : tab === 1 ? renderTableButton() : renderJSONViewer();
    }
    return tab === 0 ? renderBasicTable(tab) : renderPageTable(tab);
  }

  // renderBasicTable() organizes the data into a table based on the tab number
  function renderBasicTable(tab) {
    const body = convertTabData(state, data, tab);
    return (
      <Table responsive>
        <tbody>
          {Object.keys(body).map((k, i) => (
            <tr key={i}>
              <td className="detail-table-title">{upperCaseAndRepUnderscore(k)}</td>
              <td className="detail-table-info">{convertIfTime(k, body[k])}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    );
  }

  // renderPageTable() organizes the data into a paginated table based on the tab number
  function renderPageTable(tab) {
    let start = 0,
      end = 10,
      page = [0],
      d = data,
      ms = state.modalState,
      blk = d.block;
    if ("block" in d) {
      end = ms.page === 0 || ms.page === 1 ? 10 : ms.page * 10;
      start = end - 10;
      page = blk.transactions || page;
      d = { pageNumber: ms.Page, perPage: 10, totalPages: Math.ceil(blk.block_header.num_txs / 10), ...d };
    } else if ("account" in d) {
      page =
        tab === 1 ? convertTransactions(d.sent_transactions.results) : convertTransactions(d.rec_transactions.results);
      d = tab === 1 ? d.sent_transactions : d.rec_transactions;
    }
    return (
      <>
        <Table responsive>
          <tbody>
            <tr>
              {Object.keys(convertPaginated(convertTabData(state, data, 1)[0])).map((k, i) => (
                <td key={i} className="detail-table-row-title">
                  {upperCaseAndRepUnderscore(k)}
                </td>
              ))}
            </tr>
            {page.slice(start, end).map((item, key) => (
              <tr key={key}>
                {Object.keys(convertPaginated(item)).map((k, i) => (
                  <td key={i} className="detail-table-row-info">
                    {convertIfTime(k, item[k])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </Table>
        {pagination(d, (p) =>
          API.getModalData(ms.query, p).then((r) => {
            setState({ ...state, modalState: { ...ms, show: true, query: ms.query, page: p, data: r } });
          }),
        )}
      </>
    );
  }

  // renderJSONViewer() renders a raw json display
  function renderJSONViewer() {
    return <JsonViewer rootName={"result"} defaultInspectDepth={1} value={convertTabData(state, data, 2)} />;
  }

  // renderTableButtons() renders a button to display the account
  function renderTableButton() {
    return (
      <Button
        className="open-acc-details-btn"
        variant="outline-secondary"
        onClick={() => setState({ ...state, modalState: { ...state.modalState, accOnly: true } })}
      >
        Open Account Details
      </Button>
    );
  }

  let toCNPYFields = ["amount", "staked_amount"];

  // return the Modal
  return (
    <Modal size="xl" show={state.modalState.show} onHide={resetState}>
      <Modal.Header closeButton />
      <Modal.Body className="modal-body">
        {/* TITLE */}
        <h3 className="modal-header">
          <div className="modal-header-icon" />
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
                    <Truncate text={String(toCNPYFields.includes(k) ? toCNPY(cards[k]) : cards[k])} />
                  </div>
                  <img className="copy-img" src="./copy.png" alt="copy" />
                </Card.Body>
              </Card>,
              cards[k],
              i,
              "top",
            );
          })}
        </CardGroup>
        {/* TABS */}
        <Tabs defaultActiveKey="0" id="fill-tab-example" className="mb-3" fill>
          {[...Array(3)].map((_, i) => (
            <Tab key={i} tabClassName="rb-tab" eventKey={i} title={getTabTitle(state, data, i)}>
              {renderTab(i)}
            </Tab>
          ))}
        </Tabs>
      </Modal.Body>
    </Modal>
  );
}
