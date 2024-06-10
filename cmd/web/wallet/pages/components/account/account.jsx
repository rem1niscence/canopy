import {useState} from "react";
import JsonView from "@uiw/react-json-view";
import Truncate from 'react-truncate-inside';
import {Button, Card, Col, Form, InputGroup, Modal, Row, Table, ToastContainer, Toast, Spinner} from "react-bootstrap";
import {KeystoreGet, KeystoreImport, KeystoreNew, TxEditStake, TxPause, TxSend, TxStake, TxUnpause, TxUnstake} from "@/pages/components/api";
import {copy, formatNumber, getFormInputs, objEmpty, onFormSubmit, renderToast, withTooltip} from "@/pages/components/util";

export default function Accounts({keygroup, account, validator}) {
    const [state, setState] = useState(
        {
            showModal: false, txType: "send", txResult: {}, showSubmit: true, showPKModal: false,
            showNewModal: false, pk: {}, toast: "", showSpinner: false
        }
    ), acc = account.account

    const reset = () => {
        setState({...state, pk: {}, txResult: {}, showSubmit: true, showModal: false, showPKModal: false, showNewModal: false})
    }

    const showModal = (t) => {
        setState({...state, showModal: true, txType: t})
    }

    const getAccountType = () => (
        Object.keys(validator).length === 0 || validator.address === validator.output ? "CUSTODIAL" : "NON-CUSTODIAL"
    )

    const getValidatorAmount = () => (
        validator["staked_amount"] == null ? "0.00" : formatNumber(validator["staked_amount"])
    )

    const getStakedStatus = () => {
        if (!validator.address) {
            return "UNSTAKED"
        } else if (validator["unstaking_time"]) {
            return "UNSTAKING"
        } else {
            return "STAKED"
        }
    }

    const onPKFormSubmit = (e) => onFormSubmit(state, e, (r) => KeystoreGet(r.sender, r.password).then((r) => {
        setState({...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r})
    }))

    const onNewPKFormSubmit = (e) => onFormSubmit(state, e, (r) => KeystoreNew(r.password).then((r) => {
        setState({...state, showSubmit: Object.keys(state.txResult).length === 0, pk: r})
    }))


    const onImportOrGenerateSubmit = (e) => onFormSubmit(state, e, (r) => {
        if (r["private_key"]) {
            void KeystoreImport(r["private_key"], r.password).then((_) => setState({...state, showSpinner: false}))
        } else {
            void KeystoreNew(r.password).then((_) => setState({...state, showSpinner: false}))
        }
    })

    const onTxFormSubmit = (e) => onFormSubmit(state, e, (r) => {
        const submit = Object.keys(state.txResult).length !== 0
        switch (state.txType) {
            case "send":
                TxSend(r.sender, r.recipient, Number(r.amount), Number(r.sequence),
                    Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            case "stake":
                TxStake(r.sender, r["net_address"], Number(r.amount), r.output, Number(r.sequence),
                    Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            case "edit-stake":
                TxEditStake(r.sender, r["net_address"], Number(r.amount), r.output, Number(r.sequence),
                    Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            case "unstake":
                TxUnstake(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            case "pause":
                TxPause(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            case "unpause":
                TxUnpause(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setState({...state, showSubmit: !submit, txResult: result})
                })
                return
            default:
                return
        }
    })

    const renderSubmitBtn = (text, variant = "outline-secondary", id = "pk-button") => (
        <Button id={id} variant={variant} type="submit">{text}</Button>
    )

    const renderCloseBtn = (onClick = reset) => (
        <Button variant={"secondary"} onClick={onClick}>Close</Button>
    )

    const renderButtons = (type) => {
        switch (type) {
            case "import-or-generate":
                return renderSubmitBtn("Import or Generate Key")
            case "new-pk":
                return <>
                    {renderSubmitBtn("Generate New Key")}
                    {renderCloseBtn(reset)}
                </>
            case "reveal-pk":
                return <>
                    {renderSubmitBtn("Get Private Key", "outline-danger")}
                    {renderCloseBtn(reset)}
                </>
            default:
                if (Object.keys(state.txResult).length === 0) {
                    return <>
                        {renderSubmitBtn("Generate Transaction")}
                        {renderCloseBtn()}
                    </>
                } else {
                    const s = state.showSubmit ? renderSubmitBtn("Submit Transaction", "outline-danger") : <></>
                    return <>
                        {s}
                        {renderCloseBtn()}
                    </>
                }
        }
    }

    const renderJSONViewer = () => {
        let emptyPK = objEmpty(state.pk), emptyTxRes = objEmpty(state.txResult)
        if (emptyPK && emptyTxRes) {
            return <></>
        }
        return <JsonView value={emptyPK ? state.txResult : state.pk} shortenTextAfterLength={100} displayDataTypes={false}/>
    }

    const renderAccSumTabCol = (detail, i) => (
        withTooltip(
            <td onClick={() => copy(state, setState, detail)}>
                <img className="account-summary-info-content-image" src="./copy.png"/>
                <div className="account-summary-info-table-column">
                    <Truncate text={detail}/>
                </div>
            </td>, detail, i, "top")
    )

    const renderForm = (v, i) => {
        return Object.keys(state.txResult).length === 0 ? (
            <InputGroup key={i} className="mb-3" size="lg">
                {withTooltip(
                    <InputGroup.Text className="input-text">{v.inputText}</InputGroup.Text>, v.tooltip, i, "auto"
                )}
                <Form.Control placeholder={v.placeholder} required={v.required} defaultValue={v.defaultValue} min={0}
                              type={v.type} minLength={v.minLength} maxLength={v.maxLength} aria-label={v.label}/>
            </InputGroup>
        ) : <></>
    }

    const renderModal = (show, title, txType, onFormSub, acc, val, onHide, btnType) => {
        return (
            <Modal show={show} size="lg" onHide={onHide}>
                <Form onSubmit={onFormSub}>
                    <Modal.Header>
                        <Modal.Title>
                            {title}
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body className="modal-body">
                        {getFormInputs(txType, acc, val).map((v, i) => renderForm(v, i))}
                        {renderJSONViewer()}
                        <Spinner style={{display: state.showSpinner ? "block" : "none", margin: "0 auto"}}/>
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons(btnType)}
                    </Modal.Footer>
                </Form>
            </Modal>
        )
    }


    if (!keygroup || Object.keys(keygroup).length === 0 || !account.account) {
        return renderModal(true, "UPLOAD PRIVATE OR CREATE KEY", "pass-and-pk", onImportOrGenerateSubmit)
    }

    return <>
        <div className="content-container">
            <span id="balance">{formatNumber(acc.amount)}</span>
            <span style={{fontWeight: "bold", color: "#32908f"}}>{" CNPY"}</span>
            <br/>
            <hr style={{border: "1px dashed black", borderRadius: "5px", width: "60%", margin: "0 auto"}}/>
            <br/>
            {renderModal(state.showModal, state.txType, state.txType, onTxFormSubmit, acc, validator, reset)}
            {[
                {title: "SEND", name: "send", src: "arrow-up"},
                {title: "STAKE", name: "stake", src: "stake"},
                {title: "EDIT", name: "edit-stake", src: "edit-stake"},
                {title: "UNSTAKE", name: "unstake", src: "unstake"},
                {title: "PAUSE", name: "pause", src: "pause"},
                {title: "PLAY", name: "unpause", src: "unpause"},].map((v, i) => (
                <div key={i} className="send-receive-button-container">
                    <img className="send-receive-button" onClick={() => showModal(v.name)} src={"./" + v.src + ".png"}
                         alt="send-button"/>
                    <span style={{fontSize: "10px"}}>{v.title}</span>
                </div>
            ))}
            <Row className="account-summary-container">
                {[
                    {title: "Account Type", info: getAccountType()},
                    {title: "Account Type", info: getValidatorAmount(), after: " cnpy"},
                    {title: "Staked Status", info: getStakedStatus()}].map((v, i) => (
                    <Col key={i}>
                        <Card className="account-summary-container-card">
                            <Card.Header style={{fontWeight: "100"}}>{v.title}</Card.Header>
                            <Card.Body style={{padding: "10px 10px 5px 10px"}}>
                                <Card.Title style={{fontWeight: "bold", fontSize: "14px"}}>{v.info}
                                    <span style={{fontSize: "10px", color: "#32908f"}}>{v.after}</span>
                                </Card.Title>
                            </Card.Body>
                        </Card>
                    </Col>
                ))}
            </Row>
            <br/><br/>
            {[...Array(2)].map((_, i) => {
                const title = ["Address", "Public Key"], info = [acc.address, keygroup.publicKey]
                return <>
                    <div className="account-summary-info" onClick={() => copy(state, setState, info[i])}>
                        <span className="account-summary-info-title">{title[i]}</span>
                        <div className="account-summary-info-content-container">
                            <div className="account-summary-info-content"><Truncate text={info[i]}/></div>
                            <img style={{top: "-20px"}} className="account-summary-info-content-image" src="./copy.png"/>
                        </div>
                    </div>
                    <br/>
                </>
            })}
            <br/>
            <div style={{display: account.combined.length === 0 ? "none" : ""}} className="recent-transactions-table">
                <span style={{textAlign: "center", fontWeight: "100", fontSize: "14px", color: "grey"}}>RECENT TRANSACTIONS</span>
                <Table className="table-fixed" bordered hover style={{marginTop: "10px"}}>
                    <thead>
                    <tr>
                        {["Height", "Amount", "Recipient", "Type", "Hash"].map((k, i) => (
                            <th key={i}>{k}</th>
                        ))}
                    </tr>
                    </thead>
                    <tbody>
                    {account.combined.slice(0, 5).map((v, i) => (
                        <tr key={i}>
                            <td>{v.height}</td>
                            <td>{v.transaction.msg.amount}</td>
                            {renderAccSumTabCol(v.recipient == null ? v.sender : v.recipient, i)}
                            <td>{v.message_type}</td>
                            {renderAccSumTabCol(v.tx_hash, i)}
                        </tr>
                    ))}
                    </tbody>
                </Table>
            </div>
            {renderToast(state, setState)}
            {renderModal(state.showPKModal, "Private Key", "pass-and-addr", onPKFormSubmit, acc, null, reset, "reveal-pk")}
            {renderModal(state.showNewModal, "Private Key", "pass-only", onNewPKFormSubmit, null, null, reset, "new-pk")}
            <Button id="pk-button" variant="outline-secondary" onClick={() => setState({...state, showNewModal: true})}>New Private Key</Button>
            <Button id="reveal-pk-button" variant="outline-danger" onClick={() => setState({...state, showPKModal: true})}>Reveal Private Key</Button>
        </div>
    </>
}