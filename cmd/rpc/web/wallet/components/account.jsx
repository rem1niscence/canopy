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
  TxUnstake,
} from "@/components/api";
import FormInputs from "@/components/form_inputs";
import {
  copy,
  downloadJSON,
  formatNumber,
  getFormInputs,
  numberFromCommas,
  objEmpty,
  onFormSubmit,
  renderToast,
  retryWithDelay,
  toCNPY,
  toUCNPY,
  withTooltip,
} from "@/components/util";
import { KeystoreContext } from "@/pages";
import JsonView from "@uiw/react-json-view";
import { useContext, useEffect, useRef, useState } from "react";
import { Button, Card, Col, Form, Modal, Row, Spinner, Table } from "react-bootstrap";
import Alert from "react-bootstrap/Alert";
import Truncate from "react-truncate-inside";

function Keystore() {
  const keystore = useContext(KeystoreContext);
  return keystore;
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
  { title: "VOID", name: "delete_order", src: "delete_order" },
];

// Accounts() returns the main component of this file
export default function Accounts({ keygroup, account, validator, setActiveKey }) {
  const ks = Keystore();
  const ksRef = useRef(ks);

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
      showSpinner: false,
      showAlert: false,
      alertMsg: "",
    }),
    acc = account.account;

  const stateRef = useRef(state);

  // Keep variables updated whenever they change
  useEffect(() => {
    ksRef.current = ks;
    stateRef.current = state;
  }, [ks, state]);

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
      showPKImportModal: false,
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

  // setActivePrivateKey() sets the active key to the newly added privte key if it is successfully imported
  function setActivePrivateKey(nickname, closeModal) {
    const resetState = () =>
      setState({ ...stateRef.current, showSpinner: false, ...(closeModal && { [closeModal]: false }) });

    retryWithDelay(
      () => {
        let idx = Object.keys(ksRef.current).findIndex((k) => k === nickname);
        if (idx >= 0) {
          setActiveKey(idx);
          resetState();
        } else {
          throw new Error("failed to find key");
        }
      },
      resetState,
      10,
      1000,
      false,
    );
  }

  // onPKFormSubmit() handles the submission of the private key form and updates the state with the retrieved key
  function onPKFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) =>
      KeystoreGet(r.sender, r.password, r.nickname).then((r) => {
        setState({ ...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r });
      }),
    );
  }

  // onNewPKFormSubmit() handles the submission of the new private key form and updates the state with the generated key
  function onNewPKFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) =>
      KeystoreNew(r.password, r.nickname).then((r) => {
        setState({ ...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r });
        setActivePrivateKey(r.nickname);
      }),
    );
  }

  // onImportOrGenerateSubmit() handles the submission of either the import or generate form and updates the state accordingly
  function onImportOrGenerateSubmit(e) {
    onFormSubmit(state, e, ks, (r) => {
      if (r.private_key) {
        void KeystoreImport(r.private_key, r.password, r.nickname).then((_) => {
          setState({ ...state, showSpinner: true });
          setActivePrivateKey(r.nickname, "showPKImportModal");
        });
      } else {
        void KeystoreNew(r.password, r.nickname).then((_) => {
          setState({ ...state, showSpinner: true });
          setActivePrivateKey(r.nickname, "showPKImportModal");
        });
      }
    });
  }

  // onTxFormSubmit() handles transaction form submissions based on transaction type
  function onTxFormSubmit(e) {
    onFormSubmit(state, e, ks, (r) => {
      const submit = Object.keys(state.txResult).length !== 0;
      // Mapping transaction types to their respective functions

      const amount = toUCNPY(numberFromCommas(r.amount));
      const fee = r.fee ? toUCNPY(numberFromCommas(r.fee)) : 0;
      const receiveAmount = toUCNPY(numberFromCommas(r.receiveAmount));

      const txMap = {
        send: () => TxSend(r.sender, r.recipient, amount, r.memo, fee, r.password, submit),
        stake: () =>
          TxStake(
            r.sender,
            r.pubKey,
            r.committees,
            r.net_address,
            amount,
            r.delegate,
            r.earlyWithdrawal,
            r.output,
            r.signer,
            r.memo,
            fee,
            r.password,
            submit,
          ),
        "edit-stake": () =>
          TxEditStake(
            r.sender,
            r.committees,
            r.net_address,
            amount,
            r.earlyWithdrawal,
            r.output,
            r.signer,
            r.memo,
            fee,
            r.password,
            submit,
          ),
        unstake: () => TxUnstake(r.sender, r.signer, r.memo, fee, r.password, submit),
        pause: () => TxPause(r.sender, r.signer, r.memo, fee, r.password, submit),
        unpause: () => TxUnpause(r.sender, r.signer, r.memo, fee, r.password, submit),
        create_order: () =>
          TxCreateOrder(
            r.sender,
            r.committeeId,
            amount,
            receiveAmount,
            r.receiveAddress,
            r.memo,
            fee,
            r.password,
            submit,
          ),
        buy_order: () => TxBuyOrder(r.sender, r.receiveAddress, numberFromCommas(r.orderId), fee, r.password, submit),
        edit_order: () =>
          TxEditOrder(
            r.sender,
            r.committeeId,
            numberFromCommas(r.orderId),
            amount,
            receiveAmount,
            r.receiveAddress,
            r.memo,
            fee,
            r.password,
            submit,
          ),
        delete_order: () => TxDeleteOrder(r.sender, r.committeeId, r.orderId, r.memo, fee, r.password, submit),
      };

      const txFunction = txMap[state.txType];
      if (txFunction) {
        setState({ ...state, showAlert: false });
        txFunction()
          .then((result) => {
            setState({ ...state, showSubmit: !submit, txResult: result, showAlert: false });
          })
          .catch((e) => {
            setState({
              ...state,
              showAlert: true,
              alertMsg: "Transaction failed. Please verify the fields and try again.",
            });
          });
      }
    });
  }

  // if no private key is preset
  if (!keygroup || Object.keys(keygroup).length === 0 || !account.account) {
    return (
      <RenderModal
        show={true}
        title={"UPLOAD PRIVATE OR CREATE KEY"}
        txType={"pass-nickname-and-pk"}
        onFormSub={onImportOrGenerateSubmit}
        keyGroup={null}
        account={null}
        validator={null}
        onHide={null}
        btnType={"import-or-generate"}
        state={state}
        closeOnClick={resetState}
        keystore={ks}
      />
    );
  }
  // return the main component
  return (
    <div className="content-container">
      <span id="balance">{formatNumber(acc.amount)}</span>
      <span style={{ fontWeight: "bold", color: "#32908f" }}>{" CNPY"}</span>
      <br />
      <hr style={{ border: "1px dashed black", borderRadius: "5px", width: "60%", margin: "0 auto" }} />
      <br />
      <RenderModal
        show={state.showModal}
        title={state.txType}
        txType={state.txType}
        onFormSub={onTxFormSubmit}
        keyGroup={keygroup}
        account={acc}
        validator={validator}
        onHide={resetState}
        state={state}
        closeOnClick={resetState}
        keystore={ks}
        showAlert={state.showAlert}
        alertMsg={state.alertMsg}
      />
      {transactionButtons.map((v, i) => (
        <ActionButton key={i} v={v} i={i} showModal={showModal} />
      ))}
      <Row className="account-summary-container">
        {[
          { title: "Account Type", info: getAccountType() },
          { title: "Stake Amount", info: getValidatorAmount(), after: " cnpy" },
          { title: "Staked Status", info: getStakedStatus() },
        ].map((v, i) => (
          <RenderAccountInfo key={i} v={v} i={i} />
        ))}
      </Row>
      <br />
      <br />
      {[
        { title: "Nickname", info: keygroup.keyNickname },
        { title: "Address", info: keygroup.keyAddress },
        { title: "Public Key", info: keygroup.publicKey },
      ].map((v, i) => (
        <KeyDetail key={i} title={v.title} info={v.info} state={state} setState={setState} />
      ))}
      <br />
      <RenderTransactions account={account} state={state} setState={setState} />
      {renderToast(state, setState)}
      <RenderModal
        show={state.showPKModal}
        title={"Private Key"}
        txType={"pass-and-addr"}
        onFormSub={onPKFormSubmit}
        keyGroup={keygroup}
        account={acc}
        validator={null}
        onHide={resetState}
        state={state}
        closeOnClick={resetState}
        btnType={"reveal-pk"}
        keystore={ks}
      />
      <RenderModal
        show={state.showPKImportModal}
        title={"Private Key"}
        txType={"pass-nickname-and-pk"}
        onFormSub={onImportOrGenerateSubmit}
        keyGroup={keygroup}
        account={acc}
        validator={null}
        onHide={resetState}
        state={state}
        closeOnClick={resetState}
        btnType={"import-pk"}
        keystore={ks}
      />
      <RenderModal
        show={state.showNewModal}
        title={"Private Key"}
        txType={"pass-and-nickname"}
        onFormSub={onNewPKFormSubmit}
        keyGroup={null}
        account={null}
        validator={null}
        onHide={resetState}
        state={state}
        closeOnClick={resetState}
        btnType={"new-pk"}
        keystore={ks}
      />
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
        id="import-pk-button"
        variant="outline-secondary"
        onClick={() => {
          downloadJSON(ks, "keystore");
        }}
      >
        Download Keys
      </Button>

      <Button id="reveal-pk-button" variant="outline-danger" onClick={() => setState({ ...state, showPKModal: true })}>
        Reveal Private Key
      </Button>
    </div>
  );
}

// renderKeyDetail() creates a clickable summary info box with a copy functionality
function KeyDetail({ i, title, info, state, setState }) {
  return (
    <div key={i} className="account-summary-info" onClick={() => copy(state, setState, info)}>
      <span className="account-summary-info-title">{title}</span>
      <div className="account-summary-info-content-container">
        <div className="account-summary-info-content">
          <Truncate text={info} />
        </div>
        <img className="account-summary-info-content-image" style={{ top: "-20px" }} src="./copy.png" />
      </div>
    </div>
  );
}

// JSONViewer() returns a raw JSON viewer based on the state of pk and txResult
function JSONViewer({ pk, txResult }) {
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
// AccSumTabCol() returns an account summary table column
function AccSumTabCol({ detail, i, state, setState }) {
  return withTooltip(
    <td onClick={() => copy(state, setState, detail)}>
      <img className="account-summary-info-content-image" src="./copy.png" />
      <div className="account-summary-info-table-column">
        <Truncate text={detail} />
      </div>
    </td>,
    detail,
    i,
    "top",
  );
}

// SubmitBtn() renders a transaction submit button with customizable text, variant, and id
function SubmitBtn({ text, variant = "outline-secondary", id = "pk-button" }) {
  return (
    <Button id={id} variant={variant} type="submit">
      {text}
    </Button>
  );
}

// CloseBtn() renders a modal close button with a default onClick function
function CloseBtn({ onClick }) {
  return (
    <Button variant="secondary" onClick={onClick}>
      Close
    </Button>
  );
}

// RenderButtons() returns buttons based on the specified type
function RenderButtons({ type, state, closeOnClick }) {
  switch (type) {
    case "import-or-generate":
      return <SubmitBtn text="Import or Generate Key" />;
    case "import-pk":
      return (
        <>
          <SubmitBtn text="Import Key" variant="outline-danger" />
          <CloseBtn onClick={closeOnClick} />
        </>
      );
    case "new-pk":
      return (
        <>
          <SubmitBtn text="Generate New Key" />
          <CloseBtn onClick={closeOnClick} />
        </>
      );
    case "reveal-pk":
      return (
        <>
          <SubmitBtn text="Get Private Key" variant="outline-danger" />
          <CloseBtn onClick={closeOnClick} />
        </>
      );
    default:
      if (Object.keys(state.txResult).length === 0) {
        return (
          <>
            <SubmitBtn text={"Generate Transaction"} />
            <CloseBtn onClick={closeOnClick} />
          </>
        );
      } else {
        const s = state.showSubmit ? <SubmitBtn text="Submit Transaction" variant="outline-danger" /> : <></>;
        return (
          <>
            {s}
            {<CloseBtn onClick={closeOnClick} />}
          </>
        );
      }
  }
}

// RenderModal() returns the transaction modal
function RenderModal({
  show,
  title,
  txType,
  onFormSub,
  keyGroup,
  account,
  validator,
  onHide,
  btnType,
  state,
  closeOnClick,
  keystore,
  showAlert = false,
  alertMsg,
}) {
  return (
    <Modal show={show} size="lg" onHide={onHide}>
      <Form onSubmit={onFormSub}>
        <Modal.Header>
          <Modal.Title>{title}</Modal.Title>
        </Modal.Header>
        <Modal.Body className="modal-body">
          <FormInputs
            keygroup={keyGroup}
            fields={getFormInputs(txType, keyGroup, account, validator, keystore).map((formInput) => {
              let input = Object.assign({}, formInput);
              if (input.label === "sender") {
                input.options.sort((a, b) => {
                  if (a === account.nickname) return -1;
                  if (b === account.nickname) return 1;
                  return 0;
                });
              }
              return input;
            })}
            account={account}
            show={show}
            validator={validator}
          />
          {showAlert && <Alert variant={"danger"}>{alertMsg}</Alert>}
          <JSONViewer pk={state.pk} txResult={state.txResult} />
          <Spinner style={{ display: state.showSpinner ? "block" : "none", margin: "0 auto" }} />
        </Modal.Body>

        <Modal.Footer>
          <RenderButtons type={btnType} state={state} closeOnClick={closeOnClick} />
        </Modal.Footer>
      </Form>
    </Modal>
  );
}

// RenderActionButton() creates a button with an image and title, triggering a modal on click
function ActionButton({ v, i, showModal }) {
  return (
    <div key={i} className="send-receive-button-container">
      <img className="send-receive-button" onClick={() => showModal(v.name)} src={`./${v.src}.png`} alt={v.title} />
      <span style={{ fontSize: "10px" }}>{v.title}</span>
    </div>
  );
}

// RenderAccountInfo() generates a card displaying account summary details
function RenderAccountInfo({ v, i }) {
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

// RenderTransactions() displays a table of recent transactions based on account data
function RenderTransactions({ account, state, setState }) {
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
              <td>{toCNPY(v.transaction.msg.amount) || toCNPY(v.transaction.msg.AmountForSale) || "N/A"}</td>
              <AccSumTabCol detail={v.recipient ?? v.sender ?? v.address} i={i} state={state} setState={setState} />
              <td>{v.message_type || v.transaction.type}</td>
              <AccSumTabCol detail={v.tx_hash} i={i + 1} state={state} setState={setState} />
              <td>{v.status ?? ""}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    </div>
  );
}
