import {
    Button,
    Card,
    Col,
    Form,
    InputGroup,
    Modal,
    OverlayTrigger,
    Tooltip,
    Row,
    Table,
    ToastContainer, Toast, Spinner
} from "react-bootstrap";
import Truncate from 'react-truncate-inside';
import {useState} from "react";
import {
    KeystoreGet, KeystoreImport,
    KeystoreNew,
    TxEditStake,
    TxPause,
    TxSend,
    TxStake,
    TxUnpause,
    TxUnstake
} from "@/pages/components/api";
import JsonView from "@uiw/react-json-view";

function numberWithCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function formatNumber(nString, cutoff) {
    if (nString == null) {
        return "zero"
    }
    nString /= 1000000
    if (Number(nString) < cutoff) {
        return numberWithCommas(nString)
    }
    return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 8}).format(nString)
}

function removeFromArr(idx, array) {
    array.splice(idx, 1); // 2nd parameter means remove one item only
    return array
}

function getFormInputs(account, validator, txType, passAndAddr, passOnly, passAndPKonly) {
    let amount = null
    if (txType === "edit-stake") {
        amount = validator["staked_amount"]
    }
    let address = ""
    if (account != null) {
        address = account.address
    }
    if (txType !== "send") {
        address = validator.address
    }
    if (txType === "stake" && validator.address) {
        address = "WARNING: validator already staked"
    }
    let arr = [
        {
            "placeholder": "opt: private key hex to import",
            "defaultValue": "",
            "tooltip": "the raw private key to import if blank - will generate a new key",
            "label": "private_key",
            "inputText": "key",
            "feedback": "please choose a private key to import",
            "required": false,
            "type": "password",
            "minLength": 64,
            "maxLength": 128,
        },
        {
            "placeholder": "addr submitting the tx",
            "defaultValue": address,
            "tooltip": "required: the sender of the transaction",
            "label": "sender",
            "inputText": "address",
            "feedback": "please choose an address to send the transaction from",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        {
            "placeholder": "url of the node",
            "defaultValue": validator["net_address"],
            "tooltip": "required: the url of the validator for consensus and polling",
            "label": "net_address",
            "inputText": "net-addr",
            "feedback": "please choose a net address for the validator",
            "required": true,
            "type": "text",
            "minLength": 5,
            "maxLength": 50,
        },
        {
            "placeholder": "recipient of the tx",
            "defaultValue": "",
            "tooltip": "required: the recipient of the transaction",
            "label": "recipient",
            "inputText": "recipient",
            "feedback": "please choose a recipient for the transaction",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        {
            "placeholder": "amount value for the tx",
            "defaultValue": amount,
            "tooltip": "required: the amount of currency being sent",
            "label": "amount",
            "inputText": "amount",
            "feedback": "please choose an amount for the tx",
            "required": true,
            "type": "number",
            "minLength": 1,
            "maxLength": 100,
        },
        {
            "placeholder": "output of the node",
            "defaultValue": validator.output,
            "tooltip": "required: the non-custodial address where rewards and stake is directed to",
            "label": "output",
            "inputText": "output",
            "feedback": "please choose an output address for the validator",
            "required": true,
            "type": "text",
            "minLength": 40,
            "maxLength": 40,
        },
        {
            "placeholder": "opt: sequence num of account",
            "defaultValue": "",
            "tooltip": "a sequential number that helps prevent replay attacks. blank = default",
            "label": "sequence",
            "inputText": "sequence",
            "feedback": "please choose a valid number",
            "required": false,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        {
            "placeholder": "opt: transaction fee",
            "defaultValue": "",
            "tooltip": " a small amount of CNPY deducted from the account to process any transaction blank = default fee",
            "label": "fee",
            "inputText": "txn-fee",
            "feedback": "please choose a valid number",
            "required": false,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        {
            "placeholder": "key password",
            "defaultValue": "",
            "tooltip": "the password for the private key sending the transaction",
            "label": "password",
            "inputText": "password",
            "feedback": "please choose a valid password",
            "required": true,
            "type": "password",
            "minLength": 0,
            "maxLength": 40,
        }
    ]
    if (passAndPKonly) {
        return [arr[0], arr[8]]
    }
    if (passOnly) {
        return [arr[8]]
    }
    if (passAndAddr) {
        return [arr[1], arr[7]]
    }
    switch (txType) {
        case "send":
            arr = removeFromArr(0, arr)
            arr = removeFromArr(1, arr)
            return removeFromArr(3, arr)
        case "stake":
            arr = removeFromArr(0, arr)
            return removeFromArr(2, arr)
        case "edit-stake":
            arr = removeFromArr(0, arr)
            return removeFromArr(2, arr)
        default:
            arr = removeFromArr(0, arr)
            arr = removeFromArr(1, arr)
            arr = removeFromArr(1, arr)
            arr = removeFromArr(1, arr)
            return removeFromArr(1, arr)
    }
}

function Accounts({keygroup, accountWithTxs, validator}) {
    const [showModal, setShowModal] = useState(false);
    const [txType, setTxType] = useState("send")
    const [txResult, setTxResult] = useState({})
    const [showSubmit, setShowSubmit] = useState(true)
    const [showPKModal, setShowPKModal] = useState(false);
    const [showNewPKModal, setShowNewPKModal] = useState(false);
    const [privateKey, setPrivateKey] = useState({})
    const [showToast, setShowToast] = useState(false)
    const [showSpinner, setShowSpinner] = useState(false)
    if (!keygroup || Object.keys(keygroup).length === 0) {
        return <>
            <Modal show={true} size="lg" animation={false}>
                <Form onSubmit={onImportOrGenerateSubmit}>
                    <Modal.Header>
                        <Modal.Title>
                            UPLOAD PRIVATE OR CREATE KEY
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(accountWithTxs.account, validator, txType, false, false, true).map((v, i) => (
                            <InputGroup style={{display: Object.keys(txResult).length === 0 ? "" : "none"}} key={i}
                                        className="mb-3" size="lg">
                                <OverlayTrigger placement="auto" overlay={<Tooltip>{v.tooltip}</Tooltip>}>
                                    <InputGroup.Text style={{width: "125px"}}>{v.inputText}</InputGroup.Text>
                                </OverlayTrigger>
                                <Form.Control placeholder={v.placeholder} required={v.required}
                                              defaultValue={v.defaultValue}
                                              type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength}
                                              aria-label={v.label}/>
                            </InputGroup>
                        ))}
                        {renderJSONViewer()}
                        <Spinner style={{display:showSpinner?"block":"none", margin:"0 auto"}} animation="border" role="status">
                            <span className="visually-hidden">Loading...</span>
                        </Spinner>
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons(false, false, true)}
                    </Modal.Footer>
                </Form>
            </Modal>
            );
        </>
    }
    let account = accountWithTxs.account
    const handleClose = () => {
        setShowSubmit(true)
        setTxResult({})
        setShowModal(false)
    };

    const handlePKClose = () => {
        setShowPKModal(false)
        setPrivateKey({})
    };

    const handleNewPKClose = () => {
        setShowNewPKModal(false)
        setPrivateKey({})
    };

    const handleShow = (txType) => {
        setTxType(txType)
        setShowModal(true)
    };

    function copy(detail) {
        navigator.clipboard.writeText(detail)
        setShowToast(true)
    }

    async function onPKFormSubmit(e) {
        let r = {}
        for (let i = 1; ; i++) {
            if (!e.target[i] || !e.target[i].ariaLabel) {
                break
            }
            r[e.target[i].ariaLabel] = e.target[i].value
        }
        if (Object.keys(txResult).length !== 0) {
            setShowSubmit(false)
        }
        KeystoreGet(r.sender, r.password).then(function (result) {
            setPrivateKey(result)
        })
    }

    async function onNewPKFormSubmit(e) {
        let r = {}
        for (let i = 1; ; i++) {
            if (!e.target[i] || !e.target[i].ariaLabel) {
                break
            }
            r[e.target[i].ariaLabel] = e.target[i].value
        }
        if (Object.keys(txResult).length !== 0) {
            setShowSubmit(false)
        }
        KeystoreNew(r.password).then(function (result) {
            setPrivateKey(result)
        })
    }

    async function onImportOrGenerateSubmit(e) {
        let r = {}
        for (let i = 0; ; i++) {
            if (!e.target[i] || !e.target[i].ariaLabel) {
                break
            }
            r[e.target[i].ariaLabel] = e.target[i].value
        }
        console.log(r)
        if (r["private_key"]) {
            setShowSpinner(true)
            await KeystoreImport(r["private_key"], r.password)
        } else {
            setShowSpinner(true)
            await KeystoreNew(r.password)
        }
    }

    async function onFormSubmit(e) {
        let r = {}
        for (let i = 1; ; i++) {
            if (!e.target[i] || !e.target[i].ariaLabel) {
                break
            }
            r[e.target[i].ariaLabel] = e.target[i].value
        }
        if (Object.keys(txResult).length !== 0) {
            setShowSubmit(false)
        }
        const submit = Object.keys(txResult).length !== 0
        switch (txType) {
            case "send":
                TxSend(r.sender, r.recipient, Number(r.amount), Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            case "stake":
                TxStake(r.sender, r["net_address"], Number(r.amount), r.output, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            case "edit-stake":
                TxEditStake(r.sender, r["net_address"], Number(r.amount), r.output, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            case "unstake":
                TxUnstake(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            case "pause":
                TxPause(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            case "unpause":
                TxUnpause(r.sender, Number(r.sequence), Number(r.fee), r.password, submit).then(function (result) {
                    setTxResult(result)
                })
                return
            default:
                return
        }
    }

    function renderButtons(newPK, pk, importOrGenerate) {
        if (importOrGenerate)
            return <>
                <Button id="import-pk-button" variant="outline-secondary" type="submit">
                   Import or Generate Key
                </Button>
            </>
        if (newPK) {
            return <>
                <Button id="import-pk-button" variant="outline-secondary" type="submit">
                    Generate New Key
                </Button>
                <Button variant="secondary" onClick={handleNewPKClose}>
                    Close
                </Button>
            </>
        }
        if (pk) {
            return <>
                <Button variant="outline-danger" type="submit">
                    Get Private Key
                </Button>
                <Button variant="secondary" onClick={handlePKClose}>
                    Close
                </Button>
            </>
        }
        if (Object.keys(txResult).length === 0) {
            return <>
                <Button variant="outline-secondary" type="submit">
                    Generate Transaction
                </Button>
                <Button variant="secondary" onClick={handleClose}>
                    Close
                </Button>
            </>
        } else {
            return <>
                {(() => {
                    if (showSubmit) {
                        return <Button variant="outline-danger" type="submit">
                            Submit Transaction
                        </Button>
                    }
                })()}
                <Button variant="secondary" onClick={handleClose}>
                    Close
                </Button>
            </>
        }
    }

    function renderPK() {
        if (Object.keys(privateKey).length === 0) {
            return <></>
        }
        const result = {}
        result["result"] = privateKey
        return <JsonView
            value={result}
            shortenTextAfterLength={100}
            enableClipboard={true}
            displayDataTypes={false}
        />
    }

    function renderJSONViewer() {
        if (Object.keys(txResult).length === 0) {
            return <></>
        }
        const result = {}
        let title = showSubmit ? "transaction" : "transaction-result"
        result[title] = txResult
        return <JsonView
            value={result}
            shortenTextAfterLength={100}
            enableClipboard={true}
            displayDataTypes={false}
        />
    }

    function getAccountType() {
        if (Object.keys(validator).length === 0 || validator.address === validator.output) {
            return "CUSTODIAL"
        } else {
            return "NON-CUSTODIAL"
        }
    }

    function getValidatorAmount() {
        if (validator["staked_amount"] == null) {
            return "0.00"
        } else {
            return formatNumber(validator["staked_amount"], 1000000000000000)
        }
    }

    function getStakedStatus() {
        if (Object.keys(validator).length === 0) {
            return "UNSTAKED"
        } else if (validator["unstaking_time"]) {
            return "UNSTAKING"
        } else {
            return "STAKED"
        }
    }

    return <>
        <div className="content-container">
            <span id="balance">{formatNumber(account.amount, 1000000000000)}</span>
            <span style={{fontWeight: "bold", color: "#32908f"}}>{" CNPY"}</span>
            <br/>
            <hr style={{border: "1px dashed black", borderRadius: "5px", width: "60%", margin: "0 auto"}}/>
            <br/>
            <Modal show={showModal} size="lg" onHide={handleClose} animation={false}>
                <Form onSubmit={onFormSubmit}>
                    <Modal.Header closeButton>
                        <Modal.Title>
                            {txType}
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(account, validator, txType).map((v, i) => (
                            <InputGroup style={{display: Object.keys(txResult).length === 0 ? "" : "none"}} key={i}
                                        className="mb-3" size="lg">
                                <OverlayTrigger placement="auto" overlay={<Tooltip>{v.tooltip}</Tooltip>}>
                                    <InputGroup.Text style={{width: "125px"}}>{v.inputText}</InputGroup.Text>
                                </OverlayTrigger>
                                <Form.Control placeholder={v.placeholder} required={v.required}
                                              defaultValue={v.defaultValue}
                                              type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength}
                                              aria-label={v.label}/>
                            </InputGroup>
                        ))}
                        {renderJSONViewer()}
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons()}
                    </Modal.Footer>
                </Form>
            </Modal>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("send")} src="./arrow-up.png"
                     alt="send-button"/>
                <span style={{fontSize: "10px"}}>SEND</span>
            </div>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("stake")} src="./stake.png"
                     alt="stake-button"/>
                <span style={{fontSize: "10px"}}>STAKE</span>
            </div>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("edit-stake")} src="./edit_stake.png"
                     alt="edit-stake-button"/>
                <span style={{fontSize: "10px"}}>EDIT</span>
            </div>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("unstake")} src="./unstake.png"
                     alt="unstake-button"/>
                <span style={{fontSize: "10px"}}>UNSTAKE</span>
            </div>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("pause")} src="./pause.png"
                     alt="pause-button"/>
                <span style={{fontSize: "10px"}}>PAUSE</span>
            </div>
            <div className="send-receive-button-container">
                <img className="send-receive-button" onClick={() => handleShow("unpause")} src="./unpause.png"
                     alt="unpause-button"/>
                <span style={{fontSize: "10px"}}>PLAY</span>
            </div>
            <Row className="account-summary-container">
                <Col>
                    <Card className="account-summary-container-card">
                        <Card.Header style={{fontWeight: "100"}}>Account Type</Card.Header>
                        <Card.Body style={{padding: "10px 10px 5px 10px"}}>
                            <Card.Title style={{fontWeight: "bold", fontSize: "14px"}}>{getAccountType()}</Card.Title>
                        </Card.Body>
                    </Card>
                </Col>
                <Col>
                    <Card className="account-summary-container-card">
                        <Card.Header style={{fontWeight: "100"}}>Staked Amount</Card.Header>
                        <Card.Body style={{padding: "10px 10px 5px 10px"}}>
                            <Card.Title style={{fontWeight: "bold", fontSize: "14px"}}>{getValidatorAmount()}<span
                                style={{fontSize: "10px", color: "#32908f"}}> cnpy</span></Card.Title>
                        </Card.Body>
                    </Card>
                </Col>
                <Col>
                    <Card className="account-summary-container-card">
                        <Card.Header style={{fontWeight: "100"}}>Staked Status</Card.Header>
                        <Card.Body style={{padding: "10px 10px 5px 10px"}}>
                            <Card.Title style={{fontWeight: "bold", fontSize: "14px"}}>{getStakedStatus()}</Card.Title>
                        </Card.Body>
                    </Card>
                </Col>
            </Row>
            <br/><br/>
            <div className="account-summary-info" onClick={() => copy(account.address)}>
                <span className="account-summary-info-title">Address</span>
                <div className="account-summary-info-content-container">
                    <div className="account-summary-info-content">{account.address}</div>
                    <img style={{top: "-20px"}} className="account-summary-info-content-image" src="./copy.png"/>
                </div>
            </div>
            <br/>
            <div className="account-summary-info" onClick={() => copy(keygroup.publicKey)}>
                <span className="account-summary-info-title">Public Key</span>
                <div className="account-summary-info-content-container">
                    <div
                        className="account-summary-info-content"><Truncate text={keygroup.publicKey}/></div>
                    <img style={{top: "-20px"}} className="account-summary-info-content-image" src="./copy.png"/>
                </div>
            </div>
            <br/><br/>
            <div style={{display: accountWithTxs.combined.length === 0 ? "none" : ""}}
                 className="recent-transactions-table">
                <span style={{textAlign: "center", fontWeight: "100", fontSize: "14px", color: "grey"}}>RECENT TRANSACTIONS</span>
                <Table className="table-fixed" bordered hover style={{marginTop: "10px"}}>
                    <thead>
                    <tr>
                        <th>Height</th>
                        <th>Amount</th>
                        <th>Recipient</th>
                        <th>Type</th>
                        <th>Hash</th>
                    </tr>
                    </thead>
                    <tbody>
                    {accountWithTxs.combined.slice(0, 5).map((v, i) => (
                        <tr key={i}>
                            <td>{v.height}</td>
                            <td>{v.transaction.msg.amount}</td>
                            <OverlayTrigger placement="top" overlay={<Tooltip>{v.recipient == null ? v.sender : v.recipient}</Tooltip>}>
                                <td onClick={()=>copy(v.recipient == null ? v.sender : v.recipient)}>
                                    <img className="account-summary-info-content-image" src="./copy.png"/>
                                    <div className="account-summary-info-table-column">
                                        <Truncate text={v.recipient == null ? v.sender : v.recipient}/>
                                    </div>
                                </td>
                            </OverlayTrigger>
                            <td>{v.message_type}</td>
                            <OverlayTrigger placement="top" overlay={<Tooltip>{v.tx_hash}</Tooltip>}>
                                <td onClick={()=>copy(v.tx_hash)}>
                                    <img className="account-summary-info-content-image" src="./copy.png"/>
                                    <div className="account-summary-info-table-column">
                                        <Truncate text={v.tx_hash}/>
                                    </div>
                                </td>
                            </OverlayTrigger>
                        </tr>
                    ))}
                    </tbody>
                </Table>
            </div>
            <ToastContainer style={{width:"100px", color:"white"}} position={"bottom-end"}>
                <Toast bg={"dark"} onClose={() => {
                    setShowToast(false)
                }} show={showToast} delay={2000} autohide>
                    <Toast.Body className={'text-white'}>
                        Copied!
                    </Toast.Body>
                </Toast></ToastContainer>
            <Modal show={showPKModal} size="lg" onHide={handlePKClose} animation={false}>
                <Form onSubmit={onPKFormSubmit}>
                    <Modal.Header closeButton>
                        <Modal.Title>
                            Private Key
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(account, validator, txType, true, false).map((v, i) => (
                            <InputGroup style={{display: Object.keys(txResult).length === 0 ? "" : "none"}} key={i}
                                        className="mb-3" size="lg">
                                <OverlayTrigger placement="auto" overlay={<Tooltip>{v.tooltip}</Tooltip>}>
                                    <InputGroup.Text style={{width: "125px"}}>{v.inputText}</InputGroup.Text>
                                </OverlayTrigger>
                                <Form.Control placeholder={v.placeholder} required={v.required}
                                              defaultValue={v.defaultValue}
                                              type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength}
                                              aria-label={v.label}/>
                            </InputGroup>
                        ))}
                        {renderPK()}
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons(false, true)}
                    </Modal.Footer>
                </Form>
            </Modal>
            <Modal show={showNewPKModal} size="lg" onHide={handleNewPKClose} animation={false}>
                <Form onSubmit={onNewPKFormSubmit}>
                    <Modal.Header closeButton>
                        <Modal.Title>
                            Private Key
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(account, validator, txType, false, true).map((v, i) => (
                            <InputGroup style={{display: Object.keys(txResult).length === 0 ? "" : "none"}} key={i}
                                        className="mb-3" size="lg">
                                <OverlayTrigger placement="auto" overlay={<Tooltip>{v.tooltip}</Tooltip>}>
                                    <InputGroup.Text style={{width: "125px"}}>{v.inputText}</InputGroup.Text>
                                </OverlayTrigger>
                                <Form.Control placeholder={v.placeholder} required={v.required}
                                              defaultValue={v.defaultValue}
                                              type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength}
                                              aria-label={v.label}/>
                            </InputGroup>
                        ))}
                        {renderPK()}
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons(true, false)}
                    </Modal.Footer>
                </Form>
            </Modal>
            <Button id="import-pk-button" variant="outline-secondary" onClick={() => {
                setShowNewPKModal(true)
            }}>New Private Key</Button>{' '}
            <Button id="reveal-pk-button" variant="outline-danger" onClick={() => {
                setShowPKModal(true)
            }}>Reveal Private Key</Button>{' '}
        </div>
    </>
}

export default Accounts;