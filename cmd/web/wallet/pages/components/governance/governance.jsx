import {
    Accordion,
    Button,
    Card,
    Carousel,
    Col,
    InputGroup,
    Modal, OverlayTrigger,
    Row,
    Table,
    Toast,
    ToastContainer
} from "react-bootstrap";
import Form from 'react-bootstrap/Form';
import Truncate from 'react-truncate-inside';
import {
    PieChart,
    Line,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    AreaChart,
    ReferenceLine, XAxis, Area, Pie, ResponsiveContainer, Cell,
} from 'recharts';
import Container from "react-bootstrap/Container";
import {
    AddVote,
    DelVote,
    Params,
    Poll,
    Proposals,
    RawTx,
    TxChangeParameter,
    TxDAOTransfer
} from "@/pages/components/api";
import {useEffect, useState} from "react";
import JsonView from "@uiw/react-json-view";

const jsonPlaceholder = `{
  "parameter_space": "consensus",
  "parameter_key": "protocol_version",
  "parameter_value": "1/150",
  "start_height": 1,
  "end_height": 100,
  "signer": "303739303732333263..."
}`

const jsonPlaceholder2 = `{
  "type": "dao_transfer",
  "msg": {
    "address": "1fe1e32edc41d688d83c14d94a8dd870a29f4da9",
    "amount": 1,
    "start_height": 1,
    "end_height": 100
  },
  "signature": {
    "public_key": "a0807d42a5adfa6ef8ac3cac37a2651e838407b20986db170c5caa88b9c0c7b77e7b3ededd75242261fa6cbc3d7b0165",
    "signature": "7396288e7004497edb4b97cb7075fb7354486e3dc25d0495f491648e23943daa2a4b23619b6c96f6f4262a445d98389a6d64693ab860bb00a570814d942b6500"
  },
  "sequence": 1,
  "fee": 10000
}`

const data = [
    {name: 'YES', value: 405},
    {name: 'NO', value: 225},
    {name: 'ABS', value: 300},
];

const COLORS = ['#7749c0', '#FFF', "#d3d3d3"];

const RADIAN = Math.PI / 180;
const renderCustomizedLabel = ({cx, cy, midAngle, innerRadius, outerRadius, percent, index}) => {
    const radius = innerRadius + (outerRadius - innerRadius) * 0.15;
    const x = cx + radius * Math.cos(-midAngle * RADIAN);
    const y = cy + radius * Math.sin(-midAngle * RADIAN);
    let vote = "YES: "
    let fill = "#ebebeb"
    if (index === 1) {
        vote = "NO: "
        fill = "#222222"
    } else if (index === 2) {
        vote = "ABS: "
        fill = "#222222"
    }
    return (
        <text style={{fontWeight: "800", fontSize: "14px"}} x={x} y={y - 5} fill={fill}
              textAnchor={x > cx ? 'start' : 'end'}>
            {vote + `${(percent * 100).toFixed(0)}%`}
        </text>
    );
};

async function queryAPI() {
    let results = {}
    results.poll = await Poll()
    results.proposals = await Proposals()
    results.params = await Params(0)
    return results
}

function removeFromArr(idx, array) {
    array.splice(idx, 1); // 2nd parameter means remove one item only
    return array
}

function getFormInputs(address, txProposalType, params) {
    let arr = [
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
            "placeholder": "amount value for the tx",
            "defaultValue": "",
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
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the category 'space' of the parameter",
            "label": "param_space",
            "inputText": "param space",
            "feedback": "please choose a space for the parameter change",
            "required": true,
            "type": "select",
            "minLength": 1,
            "maxLength": 100,
        },
        {
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the identifier of the parameter",
            "label": "param_key",
            "inputText": "param key",
            "feedback": "please choose a key for the parameter change",
            "required": true,
            "type": "select",
            "minLength": 1,
            "maxLength": 100,
        },
        {
            "placeholder": "",
            "defaultValue": "",
            "tooltip": "required: the newly proposed value of the parameter",
            "label": "param_value",
            "inputText": "param val",
            "feedback": "please choose a value for the parameter change",
            "required": true,
            "type": "text",
            "minLength": 1,
            "maxLength": 100,
        },
        {
            "placeholder": "1",
            "defaultValue": "",
            "tooltip": "required: the block when voting starts",
            "label": "start_block",
            "inputText": "start blk",
            "feedback": "please choose a height for start block",
            "required": true,
            "type": "number",
            "minLength": 0,
            "maxLength": 40,
        },
        {
            "placeholder": "100",
            "defaultValue": "",
            "tooltip": "required: the block when voting is counted",
            "label": "end_block",
            "inputText": "end blk",
            "feedback": "please choose a height for end block",
            "required": true,
            "type": "number",
            "minLength": 0,
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
    if (txProposalType === 0) {
        return removeFromArr(1, arr)
    } else {
        arr = removeFromArr(2, arr)
        arr = removeFromArr(2, arr)
        return removeFromArr(2, arr)
    }
}

function Governance({keygroup, accountWithTxs}) {
    const [txResult, setTxResult] = useState({})
    const [rawTx, setRawTx] = useState({})
    const [showModal, setShowModal] = useState(false);
    const [apiResults, setAPIResults] = useState({});
    const [paramSpace, setParamSpace] = useState("");
    const [activeKey, setActiveKey] = useState(0);
    const [activeKey2, setActiveKey2] = useState(0);
    const [txProposalType, setTxProposalType] = useState(0);
    const [toastState, setToastState] = useState({
        show: false,
        text: ""
    })
    const [voteJSON, setVoteJSON] = useState({})
    useEffect(() => {
        const interval = setInterval(() => {
            queryAPI().then(results => {
                setAPIResults(results)
            })
        }, 4000);
        return () => clearInterval(interval);
    });
    if (Object.keys(apiResults).length === 0) {
        queryAPI().then(results => {
            setAPIResults(results)
        })
        return <>Loading...</>
    }
    if (Object.entries(apiResults.poll).length === 0) {
        apiResults.poll = {
            "2cbb73b8abdacf233f4c9b081991f1692145624a95004f496a95d3cce4d492a3": {
                "proposalJSON": {
                    "parameter_space": "cons|fee|val|gov",
                    "parameter_key": "protocol_version",
                    "parameter_value": "example",
                    "start_height": 1,
                    "end_height": 1000000,
                    "signer": "4646464646464646464646464646464646464646464646464646464646464646"
                },
                "approvedPower": 1000000000000000000,
                "approvedPercent": 100,
                "rejectedPercent": 0,
                "totalVotedPercent": 100,
                "rejectedPower": 0,
                "totalVotedPower": 1000000000000000000,
                "totalPower": 1000000000000000000,
                "approveVotes": null,
                "rejectVotes": [
                    {
                        "public_key": "a0807d42a5adfa6ef8ac3cac37a2651e838407b20986db170c5caa88b9c0c7b77e7b3ededd75242261fa6cbc3d7b0165",
                        "voting_power": 1000000000000000000,
                        "net_address": "http://localhost:9000"
                    }
                ]
            }
        }
    }
    if (Object.entries(apiResults.proposals).length === 0) {
        apiResults.proposals = {
            "2cbb73b8abdacf233f4c9b081991f1692145624a95004f496a95d3cce4d492a4": {
                "proposal": {
                    "parameter_space": "cons|fee|val|gov",
                    "parameter_key": "protocol_version",
                    "parameter_value": "example",
                    "start_height": 1,
                    "end_height": 1000000,
                    "signer": "4646464646464646464646464646464646464646464646464646464646464646"
                },
                "approve": false
            }
        }
    }

    function copy(detail) {
        navigator.clipboard.writeText(detail)
        setToastState({
            show: true,
            text: "Copied!"
        })
    }

    async function addVoteAPI(json, approve) {
        await AddVote(json, approve)
        setActiveKey(10)
        setToastState({
            show: true,
            text: "Voted!"
        })
    }

    async function delVoteAPI(json) {
        await DelVote(json)
        setActiveKey(10)
        setToastState({
            show: true,
            text: "Deleted!"
        })
    }

    function renderJSONViewer() {
        if (Object.keys(txResult).length === 0) {
            return <></>
        }
        const result = {}
        result["transaction-result"] = txResult
        return <JsonView
            value={result}
            shortenTextAfterLength={100}
            enableClipboard={true}
            displayDataTypes={false}
        />
    }

    function renderButtons() {
        return <>
            <Button style={{display: Object.keys(txResult).length !== 0 ? "none" : ""}} id="import-pk-button"
                    variant="outline-secondary" type="submit">
                Generate New Proposal
            </Button>
            <Button variant="secondary" onClick={handleClose}>
                Close
            </Button>
        </>
    }

    async function createDAOTransferTx(address, amount, startBlock, endBlock, seq, fee, password) {
        await TxDAOTransfer(address, amount, startBlock, endBlock, seq, fee, password, false).then(res => {
            setTxResult(res)
        })
    }

    async function createParamChangeTx(address, paramSpace, paramKey, paramValue, startBlock, endBlock, seq, fee, password) {
        await TxChangeParameter(address, paramSpace, paramKey, paramValue, startBlock, endBlock, seq, fee, password, false).then(res => {
            setTxResult(res)
        })
    }

    const handleClose = () => {
        setTxResult({})
        setShowModal(false)
        setParamSpace("")
    };

    const handleOpen = (type) => {
        setTxResult({})
        setShowModal(true)
        setTxProposalType(type)
        setParamSpace("")
    };

    async function onFormSubmit(e) {
        let r = {}
        for (let i = 1; ; i++) {
            if (!e.target[i] || !e.target[i].ariaLabel) {
                break
            }
            r[e.target[i].ariaLabel] = e.target[i].value
        }
        if (txProposalType === 0) {
            await createParamChangeTx(r.sender, r.param_space, r.param_key, r.param_value, r.start_block, r.end_block, r.sequence, r.fee, r.password)
        } else {
            await createDAOTransferTx(r.sender, r.amount, r.start_block, r.end_block, r.sequence, r.fee, r.password)
        }
    }

    async function sendRawTx(json) {
        let result = await RawTx(json)
        console.log(result)
        result = JSON.stringify(result)
        navigator.clipboard.writeText(result)
        setActiveKey2(10)
        setToastState({
            show: true,
            text: "tx hash copied to keyboard!"
        })
    }

    return <>
        <div className="content-container" style={{paddingTop: "75px"}}>
            <img className="governance-header-image" alt="vote" src="./poll.png"/>
            <span id="propose-title">poll</span>
            <span id="propose-title" style={{fontWeight: "bold", fontSize: "18px", color: "#32908f"}}>  on CANOPY</span>
            <br/><br/>
            <hr style={{border: "1px dashed black", borderRadius: "5px", width: "80%", margin: "0 auto"}}/>
            <Carousel interval={null} style={{height: "320px", width: "80%"}} data-bs-theme="dark">
                {Array.from(Object.entries(apiResults.poll)).map((entry, idx) => {
                    const [key, val] = entry;
                    const pollData = [
                        {"name": 'YES', "value": val.approvedPower},
                        {"name": 'NO', "value": val.rejectedPower},
                    ]
                    return <Carousel.Item key={idx}>
                        <Container style={{width: "80%"}} fluid>
                            <Row>
                                <Col style={{width: "50%"}}>
                                    <br/><br/><br/>
                                    <h6 style={{width: "100%", margin: "0 auto", fontWeight: "800"}}><Truncate
                                        text={"PROPOSAL: #" + key}/>
                                    </h6>
                                    <div style={{
                                        margin: "0 auto",
                                        marginTop: "20px",
                                        marginBottom: "20px",
                                        fontSize: "14px"
                                    }}>
                                        <Form.Control style={{
                                            width: "100%",
                                            height: "200px",
                                            margin: "0 auto",
                                            fontSize: "14px",
                                            backgroundColor: "#222222",
                                            color: "white",
                                            cursor: "pointer"
                                        }} onClick={() => copy(JSON.stringify(val.proposalJSON, null, 2))}
                                                      value={JSON.stringify(val.proposalJSON, null, 2)} as="textarea"
                                                      aria-label="With textarea"/>
                                    </div>
                                </Col>
                                <Col style={{width: "50%"}}>
                                    <h6 style={{
                                        width: "100%",
                                        margin: "0 auto",
                                        position: "relative",
                                        top: "72px",
                                        fontWeight: "800"
                                    }}>
                                        {"Reporting: " + val.totalVotedPercent + "%"}
                                    </h6>
                                    <ResponsiveContainer height={385}>
                                        <PieChart style={{margin: "0 auto"}}>
                                            <Pie
                                                data={pollData}
                                                cx="50%"
                                                cy="50%"
                                                labelLine={false}
                                                label={renderCustomizedLabel}
                                                outerRadius={110}
                                                dataKey="value"
                                            >
                                                {pollData.map((entry, index) => (
                                                    <Cell
                                                        style={{
                                                            filter: `drop-shadow(0px 0px 1px black)`
                                                        }}
                                                        key={`cell-${index}`} fill={COLORS[index % COLORS.length]}/>
                                                ))}
                                            </Pie>
                                        </PieChart>
                                    </ResponsiveContainer>
                                </Col>
                            </Row>
                        </Container>
                    </Carousel.Item>
                })}
            </Carousel>

            <img className="governance-header-image" alt="vote" src="./vote.png"/>
            <span id="propose-title">vote</span>
            <span id="propose-title" style={{fontWeight: "bold", fontSize: "18px", color: "#32908f"}}>  on CANOPY</span>
            <br/><br/>
            <hr style={{border: "1px dashed black", borderRadius: "5px", width: "80%", margin: "0 auto"}}/>
            <br/><br/>
            <div style={{width: "75%", margin: "0 auto"}}>
                <Table style={{width: "500px", margin: "0 auto"}} bordered responsive hover>
                    <thead>
                    <tr>
                        <th>VOTE</th>
                        <th>PROPOSAL ID</th>
                        <th>ENDS</th>
                    </tr>
                    </thead>
                    <tbody>
                    {Array.from(Object.entries(apiResults.proposals).map((entry, idx) => {
                        const [key, value] = entry
                        return <tr key={idx}>
                            <td>{value.approve ? "YES" : "NO"}</td>
                            <td>
                                <div style={{maxWidth: "300px", margin: "0 auto"}}>
                                    <Truncate text={"#" + key}/>
                                </div>
                            </td>
                            <td>{value.proposal["end_height"]}</td>
                        </tr>
                    }))}
                    </tbody>
                </Table>
                <br/>
                <Accordion activeKey={activeKey} onSelect={setActiveKey}
                           style={{display: "block", textAlign: "center", justifyContent: "center"}}>
                    <Accordion.Item style={{margin: "0 auto", width: "50%", fontWeight: "800"}} eventKey="0">
                        <Accordion.Header>VOTE ON PROPOSAL</Accordion.Header>
                        <Accordion.Body>
                            <div style={{margin: "0 auto", marginTop: "20px", marginBottom: "20px", fontSize: "20px"}}>
                                <Form.Control style={{width: "400px", height: "200px", margin: "0 auto"}}
                                              onChange={e => setVoteJSON(e.target.value)}
                                              placeholder={jsonPlaceholder} as="textarea" aria-label="With textarea"/>
                            </div>
                            <Button className="propose-button" onClick={() => addVoteAPI(voteJSON, true)}
                                    variant="outline-dark">APPROVE</Button>
                            <Button className="propose-button" onClick={() => addVoteAPI(voteJSON, false)}
                                    variant="outline-dark">REJECT</Button>
                            <Button className="propose-button" onClick={() => delVoteAPI(voteJSON)}
                                    variant="outline-dark">DELETE</Button>
                        </Accordion.Body>
                    </Accordion.Item>
                </Accordion>
                <ToastContainer style={{width: "100px", color: "white"}} position={"bottom-end"}>
                    <Toast bg={"dark"} onClose={() => {
                        setToastState({
                            show: false,
                            text: ""
                        })
                    }} show={toastState.show} delay={2000} autohide>
                        <Toast.Body className={'text-white'}>
                            {toastState.text}
                        </Toast.Body>
                    </Toast></ToastContainer>
            </div>
            <br/>
            <img className="governance-header-image" alt="propose" src="./proposal.png"/>
            <span id="propose-title">propose</span>
            <span id="propose-title" style={{fontWeight: "bold", fontSize: "18px", color: "#32908f"}}> on CANOPY</span>
            <br/><br/>
            <hr style={{border: "1px dashed black", borderRadius: "5px", width: "80%", margin: "0 auto"}}/>
            <br/>
            <Accordion activeKey={activeKey2} onSelect={i=> {
                setActiveKey2(i)
            }} style={{display: "block", textAlign: "center", justifyContent: "center"}}>
                <Accordion.Item style={{margin: "0 auto", width: "50%", fontWeight: "800"}} eventKey="0">
                    <Accordion.Header>SUBMIT EXISTING PROPOSAL</Accordion.Header>
                    <Accordion.Body>
                        <div style={{margin: "0 auto", marginTop: "20px", marginBottom: "20px", fontSize: "20px"}}>
                            <Form.Control onChange={e => setRawTx(e.target.value)} style={{width: "400px", height: "200px", margin: "0 auto"}}
                                          placeholder={jsonPlaceholder2} as="textarea" aria-label="With textarea"/>
                        </div>
                        <Button className="propose-button" onClick={() => sendRawTx(rawTx)}
                                variant="outline-dark">SUBMIT</Button>
                    </Accordion.Body>
                </Accordion.Item>
            </Accordion>
            <Button className="propose-button" onClick={() => handleOpen(0)} variant="outline-dark">New Protocol
                Change</Button>
            <Button className="propose-button" onClick={() => handleOpen(1)} variant="outline-dark">New Treasury
                Subsidy</Button>
            <Modal show={showModal} size="lg" onHide={handleClose} animation={false}>
                <Form onSubmit={onFormSubmit}>
                    <Modal.Header closeButton>
                        <Modal.Title>
                            {txProposalType === 0 ? "Change Parameter" : "Treasury Subsidy"}
                        </Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(accountWithTxs.account.address, txProposalType, apiResults.params).map((v, i) => {
                            if (v.type === "select") {
                                switch (v.label) {
                                    case "param_key":
                                        let paramKeys = []
                                        if (apiResults.params[paramSpace] != null) {
                                            paramKeys = Object.keys(apiResults.params[paramSpace])
                                        }
                                        return <InputGroup
                                            style={{display: Object.keys(txResult).length === 0 ? "" : "none"}}
                                            key={i}
                                            className="mb-3" size="lg"><Form.Select size="lg" aria-label={v.label}>
                                            <option>param key</option>
                                            {paramKeys.map((value, idx) => {
                                                return <option key={idx} value={value}>{value}</option>
                                            })}
                                        </Form.Select></InputGroup>
                                    case "param_space":
                                        return <InputGroup
                                            style={{display: Object.keys(txResult).length === 0 ? "" : "none"}}
                                            key={i}
                                            className="mb-3" size="lg">
                                            <Form.Select size="lg" onChange={e => setParamSpace(e.target.value)}
                                                         aria-label={v.label}>
                                                <option>param space</option>
                                                <option value="Consensus">consensus</option>
                                                <option value="Validator">validator</option>
                                                <option value="Governance">governance</option>
                                                <option value="Fee">fee</option>
                                            </Form.Select></InputGroup>
                                }
                            }
                            return <InputGroup style={{display: Object.keys(txResult).length === 0 ? "" : "none"}}
                                               key={i}
                                               className="mb-3" size="lg">
                                <OverlayTrigger placement="auto" overlay={<Tooltip>{v.tooltip}</Tooltip>}>
                                    <InputGroup.Text style={{width: "125px"}}>{v.inputText}</InputGroup.Text>
                                </OverlayTrigger>
                                <Form.Control placeholder={v.placeholder} required={v.required}
                                              defaultValue={v.defaultValue}
                                              type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength}
                                              aria-label={v.label}/>
                            </InputGroup>
                        })}
                        {renderJSONViewer()}
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons()}
                    </Modal.Footer>
                </Form>
            </Modal>
            <br/><br/>
        </div>
    </>
}

export default Governance;