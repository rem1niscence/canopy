import { useState, useEffect, useContext } from "react";
import JsonView from "@uiw/react-json-view";
import Truncate from "react-truncate-inside";
import { Button, Card, Col, Form, InputGroup, Modal, Row, Spinner, Table } from "react-bootstrap";
import {
  KeystoreGet,
  KeystoreImport,
  KeystoreNew,
  TxBuyOrder,
  TxCreateOrder,
  TxDeleteOrder,
  TxEditOrder,
  TxEditStake,
  TxPause,
  TxSend,
  TxStake,
  TxUnpause,
  TxUnstake
} from "@/components/api";
import {
  copy,
  formatNumber,
  formatNumberInput,
  getFormInputs,
  numberFromCommas,
  objEmpty,
  onFormSubmit,
  renderToast,
  sanitizeInput,
  withTooltip
} from "@/components/util";
import { KeystoreContext } from "@/pages";

function Keystore() {
  const keystore = useContext(KeystoreContext);
  return keystore
}

// transactionButtons defines the icons for the transactions
const transactionButtons = [
  { title: "SEND", name: "send", src: "arrow-up" },
  { title: "STAKE", name: "stake", src: "stake" },
  { title: "EDIT", name: "edit-stake", src: "edit-stake" },
  { title: "UNSTAKE", name: "unstake", src: "unstake" },
  { title: "PAUSE", name: "pause", src: "pause" },
  { title: "PLAY", name: "unpause", src: "unpause" },
  { title: "SWAP", name: "create_order", src: "swap" },
  { title: "LOCK", name: "buy_order", src: "buy" },
  { title: "REPRICE", name: "edit_order", src: "edit_order" },
  { title: "VOID", name: "delete_order", src: "delete_order" }
];

// Accounts() returns the main component of this file
export default function Accounts({ keygroup, account, validator }) {
  const ks = Keystore()
  const [state, setState] = useState({
      showModal: false,
      txType: "send",
      txResult: {},
      showSubmit: true,
      showPKModal: false,
      showPKImportModal: false,
      showNewModal: false,
      pk: {},
      toast: "",
      showSpinner: false
    }),
    acc = account.account;

  // resetState() resets the state back to its initial
  function resetState() {
    setState({
      ...state,
      pk: {},
      txResult: {},
      showSubmit: true,
      showModal: false,
      showPKModal: false,
      showNewModal: false,
      showPKImportModal: false
    });
  }

  // showModal() makes the modal visible
  function showModal(t) {
    setState({ ...state, showModal: true, txType: t });
  }

  // getAccountType() returns the type of validator account (custodial / non-custodial)
  function getAccountType() {
    return Object.keys(validator).length === 0 || validator.address === validator.output
      ? "CUSTODIAL"
      : "NON-CUSTODIAL";
  }

  // getValidatorAmount() returns the formatted staked amount of the validator
  function getValidatorAmount() {
    return validator.staked_amount == null ? "0.00" : formatNumber(validator.staked_amount);
  }

  // getStakedStatus() returns the staking status of the validator
  function getStakedStatus() {
    switch (true) {
      case !validator.address:
        return "UNSTAKED";
      case validator.unstaking_height !== 0:
        return "UNSTAKING";
      case validator.max_paused_height !== 0:
        return "PAUSED";
      case validator.delegate:
        return "DELEGATING";
      default:
        return "STAKED";
    }
  }

  // onPKFormSubmit() handles the submission of the private key form and updates the state with the retrieved key
  function onPKFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) =>
      KeystoreGet(r.sender, r.password, r.nickname).then((r) => {
        setState({ ...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r });
      })
    );
  }

  // onNewPKFormSubmit() handles the submission of the new private key form and updates the state with the generated key
  function onNewPKFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) =>
      KeystoreNew(r.password, r.nickname).then((r) => {
        setState({ ...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r });
      })
    );
  }

  // onImportOrGenerateSubmit() handles the submission of either the import or generate form and updates the state accordingly
  function onImportOrGenerateSubmit(e) {
    onFormSubmit(state, e, ks, (r) => {
      if (r.private_key) {
        void KeystoreImport(r.private_key, r.password, r.nickname).then((_) => setState({ ...state, showSpinner: false }));
      } else {
        void KeystoreNew(r.password, r.nickname).then((_) => setState({ ...state, showSpinner: false }));
      }
    });
  }

  // onTxFormSubmit() handles transaction form submissions based on transaction type
  function onTxFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) => {
      const submit = Object.keys(state.txResult).length !== 0;
      // Mapping transaction types to their respective functions
      const txMap = {
        send: () =>
          TxSend(
            r.sender,
            r.recipient,
            numberFromCommas(r.amount),
            r.memo,
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        stake: () =>
          TxStake(
            r.sender,
            r.pubKey,
            r.committees,
            r.net_address,
            numberFromCommas(r.amount),
            r.delegate,
            r.earlyWithdrawal,
            r.output,
            r.signer,
            r.memo,
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        "edit-stake": () =>
          TxEditStake(
            r.sender,
            r.committees,
            r.net_address,
            numberFromCommas(r.amount),
            r.earlyWithdrawal,
            r.output,
            r.signer,
            r.memo,
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        unstake: () => TxUnstake(r.sender, r.signer, r.memo, numberFromCommas(r.fee), r.password, submit),
        pause: () => TxPause(r.sender, r.signer, r.memo, numberFromCommas(r.fee), r.password, submit),
        unpause: () => TxUnpause(r.sender, r.signer, r.memo, numberFromCommas(r.fee), r.password, submit),
        create_order: () =>
          TxCreateOrder(
            r.sender,
            r.committeeId,
            numberFromCommas(r.amount),
            numberFromCommas(r.receiveAmount),
            r.receiveAddress,
            r.memo,
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        buy_order: () =>
          TxBuyOrder(
            r.sender,
            r.receiveAddress,
            numberFromCommas(r.orderId),
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        edit_order: () =>
          TxEditOrder(
            r.sender,
            r.committeeId,
            numberFromCommas(r.orderId),
            numberFromCommas(r.amount),
            numberFromCommas(r.receiveAmount),
            r.receiveAddress,
            r.memo,
            numberFromCommas(r.fee),
            r.password,
            submit
          ),
        delete_order: () =>
          TxDeleteOrder(r.sender, r.committeeId, r.orderId, r.memo, numberFromCommas(r.fee), r.password, submit)
      };

      const txFunction = txMap[state.txType];
      if (txFunction) {
        txFunction().then((result) => {
          setState({ ...state, showSubmit: !submit, txResult: result });
        });
      }
    });
  }

  // renderSubmitBtn() renders a transaction submit button with customizable text, variant, and id
  function renderSubmitBtn(text, variant = "outline-secondary", id = "pk-button") {
    return (
      <Button id={id} variant={variant} type="submit">
        {text}
      </Button>
    );
  }

  // renderCloseBtn() renders a modal close button with a default onClick function
  function renderCloseBtn(onClick = resetState) {
    return (
      <Button variant="secondary" onClick={onClick}>
        Close
      </Button>
    );
  }

  // renderButtons() returns buttons based on the specified type
  function renderButtons(type) {
    switch (type) {
      case "import-or-generate":
        return renderSubmitBtn("Import or Generate Key");
      case "import-pk":
        return (
          <>
            {renderSubmitBtn("Import Key", "outline-danger")}
            {renderCloseBtn(resetState)}
          </>
        );
      case "new-pk":
        return (
          <>
            {renderSubmitBtn("Generate New Key")}
            {renderCloseBtn(resetState)}
          </>
        );
      case "reveal-pk":
        return (
          <>
            {renderSubmitBtn("Get Private Key", "outline-danger")}
            {renderCloseBtn(resetState)}
          </>
        );
      default:
        if (Object.keys(state.txResult).length === 0) {
          return (
            <>
              {renderSubmitBtn("Generate Transaction")}
              {renderCloseBtn()}
            </>
          );
        } else {
          const s = state.showSubmit ? renderSubmitBtn("Submit Transaction", "outline-danger") : <></>;
          return (
            <>
              {s}
              {renderCloseBtn()}
            </>
          );
        }
    }
  }

  // renderJSONViewer() returns a raw JSON viewer based on the state of pk and txResult
  function renderJSONViewer() {
    const { pk, txResult } = state;
    const isEmptyPK = objEmpty(pk);
    const isEmptyTxRes = objEmpty(txResult);

    if (isEmptyPK && isEmptyTxRes) return <></>;
    return (
      <JsonView
        value={isEmptyPK ? { result: txResult } : { result: pk }}
        shortenTextAfterLength={100}
        displayDataTypes={false}
      />
    );
  }

  // renderAccSumTabCol() returns an account summary table column
  function renderAccSumTabCol(detail, i) {
    return withTooltip(
      <td onClick={() => copy(state, setState, detail)}>
        <img className="account-summary-info-content-image" src="./copy.png" />
        <div className="account-summary-info-table-column">
          <Truncate text={detail} />
        </div>
      </td>,
      detail,
      i,
      "top"
    );
  }

  // renderForm() returns a form input group for the transaction execution
  function renderForm(fields, show) {
    // Manage all form input values in a single state object to allow for dynamic form generation
    // and state management
    const [formValues, setFormValues] = useState({});

    // sets the default form values based on the fields every time the modal is opened
    useEffect(() => {
      const initialValues = fields.reduce((form, field) => {
        const value = field.defaultValue || "";
        form[field.label] = field.type === "number" ? formatNumberInput(value.toString()) : value;
        return form;
      }, {});

      setFormValues(initialValues);
    }, [show]);

    const handleInputChange = (key, value, type) => {
      setFormValues((prev) => ({
        ...prev,
        [key]: type === "number" ? formatNumberInput(value) : sanitizeInput(value)
      }));
    };

    const doRenderForm = (v, i) => {
      if (v.shouldNotRender && v.shouldNotRender(keygroup, account, validator)) return null;

      return (
        <Form.Group key={i} className="mb-3">
          <InputGroup size="lg">
            {withTooltip(<InputGroup.Text className="input-text">{v.inputText}</InputGroup.Text>, v.tooltip, i, "auto")}
            {v.type === "dropdown" ? (
              <Form.Select
                className="input-text-field"
                onChange={(e) => handleInputChange(v.label, e.target.value, v.type)}
                value={formValues[v.label]}
                aria-label={v.label}
              >
                {v.options && v.options.length > 0 ? (
                  v.options.map((key) => (
                    <option key={key} value={key}>
                      {key}
                    </option>
                  ))
                ) : (
                  <option disabled>No options available</option>
                )}
              </Form.Select>
            ) : (
            <Form.Control
              className="input-text-field"
              onChange={(e) => handleInputChange(v.label, e.target.value, v.type)}
              type={v.type == "number" ? "text" : v.type}
              value={formValues[v.label]}
              placeholder={v.placeholder}
              required={v.required}
              min={0}
              minLength={v.minLength}
              maxLength={v.maxLength}
              aria-label={v.label}
              aria-describedby="emailHelp"
            />)}
          </InputGroup>
          {v.label === "amount" ? renderAmountInput(handleInputChange, v) : null}
        </Form.Group>
      );
    };

    return fields.map(doRenderForm);
  }

  // renderAmountInput() renders the amount input with the option to set the amount to max
  function renderAmountInput(onchange, v) {
    const amount = formatNumber(account.account.amount);
    return (
      <div className="text-end">
        <Form.Text>
          Available: <span className="fw-bold">{amount} </span>
          <Button
            aria-label="max-button"
            onClick={() => onchange(v.label, Math.ceil(account.account.amount).toString(), v.type)}
            variant="link"
            bsPrefix="max-amount-btn"
          >
            MAX
          </Button>
        </Form.Text>
      </div>
    );
  }

  // renderModal() returns the transaction modal
  function renderModal(show, title, txType, onFormSub, keyGroup, acc, val, onHide, btnType) {
    return (
      <Modal show={show} size="lg" onHide={onHide}>
        <Form onSubmit={onFormSub}>
          <Modal.Header>
            <Modal.Title>{title}</Modal.Title>
          </Modal.Header>
          <Modal.Body className="modal-body">
            {renderForm(getFormInputs(txType, keyGroup, acc, val), show)}
            {renderJSONViewer()}
            <Spinner style={{ display: state.showSpinner ? "block" : "none", margin: "0 auto" }} />
          </Modal.Body>
          <Modal.Footer>{renderButtons(btnType)}</Modal.Footer>
        </Form>
      </Modal>
    );
  }

  // renderActionButton() creates a button with an image and title, triggering a modal on click
  function renderActionButton(v, i) {
    return (
      <div key={i} className="send-receive-button-container">
        <img className="send-receive-button" onClick={() => showModal(v.name)} src={`./${v.src}.png`} alt={v.title} />
        <span style={{ fontSize: "10px" }}>{v.title}</span>
      </div>
    );
  }

  // renderAccountInfo() generates a card displaying account summary details
  function renderAccountInfo(v, i) {
    return (
      <Col key={i}>
        <Card className="account-summary-container-card">
          <Card.Header style={{ fontWeight: "100" }}>{v.title}</Card.Header>
          <Card.Body style={{ padding: "10px" }}>
            <Card.Title style={{ fontWeight: "bold", fontSize: "14px" }}>
              {v.info}
              <span style={{ fontSize: "10px", color: "#32908f" }}>{v.after}</span>
            </Card.Title>
          </Card.Body>
        </Card>
      </Col>
    );
  }

  // renderKeyDetail() creates a clickable summary info box with a copy functionality
  function renderKeyDetail(v, i) {
    return (
      <div key={i} className="account-summary-info" onClick={() => copy(state, setState, v.info)}>
        <span className="account-summary-info-title">{v.title}</span>
        <div className="account-summary-info-content-container">
          <div className="account-summary-info-content">
            <Truncate text={v.info} />
          </div>
          <img className="account-summary-info-content-image" style={{ top: "-20px" }} src="./copy.png" />
        </div>
      </div>
    );
  }

  // renderTransactions() displays a table of recent transactions based on account data
  function renderTransactions() {
    return account.combined.length === 0 ? null : (
      <div className="recent-transactions-table">
        <span style={{ textAlign: "center", fontWeight: "100", fontSize: "14px", color: "grey" }}>
          RECENT TRANSACTIONS
        </span>
        <Table className="table-fixed" bordered hover style={{ marginTop: "10px" }}>
          <thead>
          <tr>
            {["Height", "Amount", "Recipient", "Type", "Hash", "Status"].map((k, i) => (
              <th key={i}>{k}</th>
            ))}
          </tr>
          </thead>
          <tbody>
          {account.combined.slice(0, 5).map((v, i) => (
            <tr key={i}>
              <td>{v.height || "N/A"}</td>
              <td>{v.transaction.msg.amount ?? v.transaction.msg.AmountForSale ?? "N/A"}</td>
              {renderAccSumTabCol(v.recipient ?? v.sender ?? v.address, i)}
              <td>{v.message_type || v.transaction.type}</td>
              {renderAccSumTabCol(v.tx_hash, i + 1)}
              <td>{v.status ?? ""}</td>
            </tr>
          ))}
          </tbody>
        </Table>
      </div>
    );
  }

  // if no private key is preset
  if (!keygroup || Object.keys(keygroup).length === 0 || !account.account) {
    return renderModal(
      true,
      "UPLOAD PRIVATE OR CREATE KEY",
      "pass-nickname-and-pk",
      onImportOrGenerateSubmit,
      null,
      null,
      null,
      null,
      "import-or-generate"
    );
  }
  // return the main component
  return (
    <>
      <div className="content-container">
        <span id="balance">{formatNumber(acc.amount)}</span>
        <span style={{ fontWeight: "bold", color: "#32908f" }}>{" CNPY"}</span>
        <br />
        <hr style={{ border: "1px dashed black", borderRadius: "5px", width: "60%", margin: "0 auto" }} />
        <br />
        {renderModal(state.showModal, state.txType, state.txType, onTxFormSubmit, keygroup, acc, validator, resetState)}
        {transactionButtons.map(renderActionButton)}
        <Row className="account-summary-container">
          {[
            { title: "Account Type", info: getAccountType() },
            { title: "Stake Amount", info: getValidatorAmount(), after: " cnpy" },
            { title: "Staked Status", info: getStakedStatus() }
          ].map(renderAccountInfo)}
        </Row>
        <br />
        <br />
        {[
          { title: "Nickname", info: keygroup.keyNickname },
          { title: "Address", info: keygroup.keyAddress },
          { title: "Public Key", info: keygroup.publicKey },
        ].map(renderKeyDetail)}
        <br />
        {renderTransactions()}
        {renderToast(state, setState)}
        {renderModal(
          state.showPKModal,
          "Private Key",
          "pass-and-addr",
          onPKFormSubmit,
          keygroup,
          acc,
          null,
          resetState,
          "reveal-pk"
        )}
        {renderModal(
          state.showPKImportModal,
          "Private Key",
          "pass-nickname-and-pk",
          onImportOrGenerateSubmit,
          keygroup,
          acc,
          null,
          resetState,
          "import-pk"
        )}
        {renderModal(
          state.showNewModal,
          "Private Key",
          "pass-and-nickname",
          onNewPKFormSubmit,
          null,
          null,
          null,
          resetState,
          "new-pk"
        )}
        <Button id="pk-button" variant="outline-secondary" onClick={() => setState({ ...state, showNewModal: true })}>
          New Private Key
        </Button>
        <Button
          id="import-pk-button"
          variant="outline-secondary"
          onClick={() => setState({ ...state, showPKImportModal: true })}
        >
          Import Private Key
        </Button>
        <Button
          id="reveal-pk-button"
          variant="outline-danger"
          onClick={() => setState({ ...state, showPKModal: true })}
        >
          Reveal Private Key
        </Button>
      </div>
    </>
  );
}
