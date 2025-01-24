import { Table } from "react-bootstrap";
import Truncate from "react-truncate-inside";
import {
  cpyObj,
  convertIfTime,
  convertNumberWCommas,
  convertTime,
  isHex,
  isNumber,
  pagination,
  upperCaseAndRepUnderscore,
  convertTx,
} from "@/components/util";

// convertValue() converts the value based on its key and handles different types
function convertValue(k, v, openModal) {
  if (k === "public_key") return <Truncate text={v} />;
  if (isHex(v) || k === "height") {
    const content = isNumber(v) ? v : <Truncate text={v} />;
    return (
      <a href="#" onClick={() => openModal(v)} style={{ cursor: "pointer" }}>
        {content}
      </a>
    );
  }
  if (k.includes("time")) return convertTime(v);
  if (isNumber(v)) return convertNumberWCommas(v);
  return convertIfTime(k, v);
}

// convertTransaction() converts a transaction item into a display object
export function convertTransaction(v) {
  let value = Object.assign({}, v);
  delete value.transaction;
  return convertTx(value);
}

// sortData() sorts table data by a given column and direction
function sortData(data, column, direction) {
  if (!column) return data;
  return [...data].sort((a, b) => {
    const aValue = a[column];
    const bValue = b[column];
    if (aValue < bValue) return direction === "asc" ? -1 : 1;
    if (aValue > bValue) return direction === "asc" ? 1 : -1;
    return 0;
  });
}

// filterData() filters table data based on the filterText
function filterData(data, filterText) {
  if (!filterText) return data;
  return data.filter((row) =>
    Object.values(row).some((value) => value?.toString().toLowerCase().includes(filterText.toLowerCase())),
  );
}

// convertBlock() processes block header, removing specific fields for table
function convertBlock(v) {
  let {
    last_quorum_certificate,
    next_validator_root,
    state_root,
    transaction_root,
    validator_root,
    last_block_hash,
    network_id,
    total_vdf_iterations,
    vdf,
    ...value
  } = cpyObj(v.block_header);
  value.num_txs = "num_txs" in v.block_header ? v.block_header.num_txs : "0";
  value.total_txs = "total_txs" in v.block_header ? v.block_header.total_txs : "0";
  return JSON.parse(JSON.stringify(value, ["height", "hash", "time", "num_txs", "total_txs", "proposer_address"], 4));
}

// convertParams() processes different consensus parameters for table structure
function convertParams(v) {
  if (!v.Consensus) return ["0"];
  let value = cpyObj(v);
  return ["Consensus", "Validator", "Fee", "Governance"].flatMap((space) =>
    Object.entries(value[space] || {}).map(([k, v]) => ({
      ParamName: k,
      ParamValue: v,
      ParamSpace: space,
    })),
  );
}

// convertOrder() transforms order details into a table-compatible convert
function convertOrder(v) {
  const exchangeRate = v.RequestedAmount / v.AmountForSale;
  return {
    Id: v.Id ?? 0,
    Chain: v.Committee,
    AmountForSale: v.AmountForSale,
    Rate: exchangeRate.toFixed(2),
    RequestedAmount: v.RequestedAmount,
    SellerReceiveAddress: v.SellerReceiveAddress,
    SellersSendAddress: v.SellersSendAddress,
    BuyerSendAddress: v.BuyerSendAddress,
    Status: "BuyerReceiveAddress" in v ? "Reserved" : "Open",
    BuyerReceiveAddress: v.BuyerReceiveAddress,
    BuyerChainDeadline: v.BuyerChainDeadline,
  };
}

// convertCommitteeSupply() calculates supply percentage for table display
function convertCommitteeSupply(v, total) {
  const percent = 100 * (v.amount / total);
  return {
    Chain: 1,
    stake_cut: `${percent}%`,
    total_restake: v.amount,
  };
}

// getHeader() returns the appropriate header for the table based on the object type
function getHeader(v) {
  if (v.type === "tx-results-page") return "Transactions";
  if (v.type === "pending-results-page") return "Pending";
  if (v.type === "block-results-page") return "Blocks";
  if (v.type === "accounts") return "Accounts";
  if (v.type === "validators") return "Validators";
  if ("Consensus" in v) return "Governance";
  if ("committee_staked" in v) return "Committees";
  return "Sell Orders";
}

// getTableBody() determines the body of the table based on the provided object type
function getTableBody(v) {
  let empty = [{ Results: "null" }];
  if ("Consensus" in v) return convertParams(v);
  if ("committee_staked" in v) return v.committee_staked.map((item) => convertCommitteeSupply(item, v.staked));
  if (!v.hasOwnProperty("type")) return v[0]?.orders?.map(convertOrder) || empty;
  if (v.results === null) return empty;
  const converters = {
    "tx-results-page": convertTransaction,
    "pending-results-page": convertTransaction,
    "block-results-page": convertBlock,
    accounts: (item) => item,
    validators: (item) => item,
  };
  let results = v.results.map(converters[v.type] || (() => []));
  return results.length === 0 ? empty : results;
}

// DTable() renders the main data table with sorting, filtering, and pagination
export default function DTable(props) {
  const { filterText, sortColumn, sortDirection, category, committee, tableData } = props.state;
  const sortedData = sortData(filterData(getTableBody(tableData), filterText), sortColumn, sortDirection);
  return (
    <div className="data-table">
      <div className="data-table-content">
        {category === 6 && (
          <input
            type="number"
            value={committee}
            min="1"
            onChange={(e) => e.target.value && props.selectTable(6, 0, Number(e.target.value))}
            className="chain-table mb-3"
          />
        )}
        <input
          type="text"
          value={filterText}
          onChange={(e) => props.setState({ ...props.state, filterText: e.target.value })}
          className="search-table mb-3"
        />
        <h5 className="data-table-head">{getHeader(tableData)}</h5>
      </div>

      <Table responsive bordered hover size="sm" className="table">
        <thead>
          <tr>
            {Object.keys(getTableBody(tableData)[0]).map((s, i) => (
              <th
                key={i}
                className="table-head"
                onClick={() => {
                  const direction = sortColumn === s && sortDirection === "asc" ? "desc" : "asc";
                  props.setState({ ...props.state, sortColumn: s, sortDirection: direction });
                }}
                style={{ cursor: "pointer" }}
              >
                {upperCaseAndRepUnderscore(s)}
                {sortColumn === s && (sortDirection === "asc" ? " ↑" : " ↓")}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sortedData.map((val, idx) => (
            <tr key={idx}>
              {Object.keys(val).map((k, i) => (
                <td key={i} className="table-col">
                  {convertValue(k, val[k], props.openModal)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </Table>

      {pagination(tableData, (i) => props.selectTable(props.state.category, i))}
    </div>
  );
}
