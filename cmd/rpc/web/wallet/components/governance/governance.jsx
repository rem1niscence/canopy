import {useEffect, useState} from "react";
import Truncate from 'react-truncate-inside';
import Container from "react-bootstrap/Container";
import JsonView from "@uiw/react-json-view";
import Form from 'react-bootstrap/Form';
import {PieChart, Pie, ResponsiveContainer, Cell,} from 'recharts';
import {Accordion, Button, Carousel, Col, InputGroup, Modal, Row, Spinner, Table} from "react-bootstrap";
import {AddVote, DelVote, Params, Poll, Proposals, RawTx, TxChangeParameter, TxDAOTransfer} from "@/components/api";
import {copy, getFormInputs, objEmpty, onFormSubmit, placeholders, renderToast, withTooltip} from "@/components/util";

export default function Governance({account: accountWithTxs}) {
    const [state, setState] = useState({
        txResult: {}, rawTx: {}, showModal: false, apiResults: {}, paramSpace: "", voteAccord: "1",
        propAccord: "1", txPropType: 0, toast: "", voteJSON: {},
    })
    const queryAPI = () => {
        Promise.all([Poll(), Proposals(), Params(0)]).then((r => {
                setState({...state, apiResults: {poll: r[0], proposals: r[1], params: r[2]}})
            }
        ))
    }
    useEffect(() => {
        return () => clearInterval(setInterval(() => queryAPI(), 4000))
    })
    if (objEmpty(state.apiResults)) {
        queryAPI()
        return <Spinner id="spinner"/>
    }
    if (objEmpty(state.apiResults.poll)) {
        state.apiResults.poll = placeholders.poll
    }
    if (objEmpty(state.apiResults.proposals)) {
        state.apiResults.proposals = placeholders.proposals
    }
    const addVoteAPI = (json, approve) => AddVote(json, approve).then(_ => setState({...state, voteAccord: "1", toast: "Voted!"}))
    const delVoteAPI = (json) => DelVote(json).then(_ => setState({...state, voteAccord: "1", toast: "Deleted!"}))
    const handleClose = () => setState({...state, paramSpace: "", txResult: {}, showModal: false})
    const handleOpen = (type) => setState({...state, txPropType: type, paramSpace: "", txResult: {}, showModal: true})
    const sendRawTx = (j) => RawTx(j).then(r => {
        console.log(r)
        copy(state, setState, r, "tx hash copied to keyboard!")
    })
    const createDAOTransferTx = (address, amount, startBlock, endBlock, seq, fee, password) => {
        TxDAOTransfer(address, amount, startBlock, endBlock, seq, fee, password, false).then(res => {
            setState({...state, txResult: res})
        })
    }
    const createParamChangeTx = (address, paramSpace, paramKey, paramValue, startBlock, endBlock, seq, fee, password) => {
        TxChangeParameter(address, paramSpace, paramKey, paramValue, startBlock, endBlock, seq, fee, password, false).then(res => {
            setState({...state, txResult: res})
        })
    }
    const onSubmit = (e) => onFormSubmit(state, e, (r) => {
        if (state.txPropType === 0) {
            createParamChangeTx(r.sender, r.param_space, r.param_key, r.param_value, r.start_block, r.end_block, r.sequence, r.fee, r.password)
        } else {
            createDAOTransferTx(r.sender, r.amount, r.start_block, r.end_block, r.sequence, r.fee, r.password)
        }
    })
    const renderJSONViewer = () => (
        objEmpty(state.txResult) ? <></> : <JsonView value={state.txResult} shortenTextAfterLength={100} displayDataTypes={false}/>
    )
    const renderButtons = () => (
        <>
            <Button style={{display: objEmpty(state.txResult) ? "" : "none"}} id="import-pk-button" variant="outline-secondary" type="submit">
                Generate New Proposal
            </Button>
            <Button variant="secondary" onClick={handleClose}>
                Close
            </Button>
        </>
    )
    const renderPiLabel = ({cx, cy, midAngle, innerRadius, outerRadius, percent, idx}) => {
        const radius = innerRadius + (outerRadius - innerRadius) * 0.15, c = idx === 0
        const x = cx + radius * Math.cos(-midAngle * Math.PI / 180)
        const y = cy + radius * Math.sin(-midAngle * Math.PI / 180)
        return (
            <text className="pi-label" x={x} y={y - 5} textAnchor={x > cx ? 'start' : 'end'} fill={c ? "#ebebeb" : "#222222"}>
                {c ? "YES: " : "NO: " + `${(percent * 100).toFixed(0)}%`}
            </text>
        )
    }
    const renderHeader = (title, img) => (
        <>
            <img className="governance-header-image" alt="vote" src={img}/>
            <span id="propose-title">{title}</span><span id="propose-subtitle">  on CANOPY</span>
            <br/><br/>
            <hr className="gov-header-hr"/>
            <br/><br/>
        </>
    )
    const renderAccord = (title, keyName, targetName, buttons, placeholder = placeholders.params) => {
        let s = {...state}, activeKey = state[keyName], ph = JSON.stringify(placeholder, null, "  ")
        const onSelect = (i) => {
            s[keyName] = i
            setState(s)
        }
        const onChange = (e) => {
            s[targetName] = e.target.value
            setState({...s, voteJSON: e.target.value})
        }
        return <Accordion className="accord" activeKey={activeKey} onSelect={onSelect}>
            <Accordion.Item className="accord-item" eventKey="0">
                <Accordion.Header>{title}</Accordion.Header>
                <Accordion.Body>
                    <Form.Control className="accord-body-container" placeholder={ph} as="textarea" onChange={onChange}/>
                    {buttons.map((k) => (
                        <Button className="propose-button" onClick={k.onClick} variant="outline-dark">{k.title}</Button>
                    ))}
                </Accordion.Body>
            </Accordion.Item>
        </Accordion>
    }
    return <>
        <div className="content-container">
            {renderHeader("poll", "./poll.png")}
            <Carousel className="poll-carousel" interval={null} data-bs-theme="dark">
                {Array.from(Object.entries(state.apiResults.poll)).map((entry, idx) => {
                    const [key, val] = entry;
                    const pollData = [
                        {"name": 'YES', "value": val.approvedPower, "color": "#7749c0"},
                        {"name": 'NO', "value": val.rejectedPower, "color": "#FFF"},
                    ]
                    return <Carousel.Item key={idx}>
                        <Container className="poll-carousel-container" fluid>
                            <Row>
                                <Col className="poll-column">
                                    <h6 className="poll-prop-name"><Truncate text={"PROPOSAL: #" + key}/></h6>
                                    <div className="poll-container">
                                        <Form.Control className="poll-form" as="textarea" value={JSON.stringify(val.proposalJSON, null, 2)}
                                                      onClick={() => copy(state, setState, JSON.stringify(val.proposalJSON, null, 2))}
                                        />
                                    </div>
                                </Col>
                                <Col className="poll-column">
                                    <h6 className="poll-prop-name" id="poll-pi-header">{"Reporting: " + val.totalVotedPercent + "%"}</h6>
                                    <ResponsiveContainer height={385}>
                                        <PieChart style={{margin: "0 auto"}}>
                                            <Pie data={pollData} cx="50%" cy="32%" labelLine={false} label={renderPiLabel} outerRadius={105}>
                                                {pollData.map((e, i) => (
                                                    <Cell className="pi-cell" key={`cell-${i}`} fill={e.color}/>
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
            {renderHeader("vote", "./vote.png")}
            <Table className="vote-table" bordered responsive hover>
                <thead>
                <tr>
                    <th>VOTE</th>
                    <th>PROPOSAL ID</th>
                    <th>ENDS</th>
                </tr>
                </thead>
                <tbody>
                {Array.from(Object.entries(state.apiResults.proposals).map((entry, idx) => {
                    const [key, value] = entry
                    return <tr key={idx}>
                        <td>{value.approve ? "YES" : "NO"}</td>
                        <td>
                            <div className="vote-table-col">
                                <Truncate text={"#" + key}/>
                            </div>
                        </td>
                        <td>{value.proposal["end_height"]}</td>
                    </tr>
                }))}
                </tbody>
            </Table>
            {renderAccord("VOTE ON PROPOSAL", "voteAccord", "voteJSON", [
                {title: "APPROVE", onClick: () => addVoteAPI(state.voteJSON, true)},
                {title: "REJECT", onClick: () => addVoteAPI(state.voteJSON, false)},
                {title: "DELETE", onClick: () => delVoteAPI(state.voteJSON)},
            ])}
            {renderToast(state, setState)}
            {renderHeader("propose", "./proposal.png")}
            {renderAccord("EXISTING PROPOSAL", "propAccord", "rawTx", [
                {title: "SUBMIT", onClick: () => sendRawTx(state.rawTx)}
            ], placeholders.rawTx)}
            <Button className="propose-button" onClick={() => handleOpen(0)} variant="outline-dark">New Protocol Change</Button>
            <Button className="propose-button" onClick={() => handleOpen(1)} variant="outline-dark">New Treasury Subsidy</Button>
            <br/><br/>
            <Modal show={state.showModal} size="lg" onHide={handleClose}>
                <Form onSubmit={onSubmit}>
                    <Modal.Header closeButton>
                        <Modal.Title>{state.txPropType === 0 ? "Change Parameter" : "Treasury Subsidy"}</Modal.Title>
                    </Modal.Header>
                    <Modal.Body style={{overflowWrap: "break-word"}}>
                        {getFormInputs(state.txPropType === 0 ? "change-param" : "dao-transfer", accountWithTxs.account).map((v, i) => {
                            let show = objEmpty(state.txResult) ? "" : "none", onChange = (e) => setState({...state, paramSpace: e.target.value})
                            if (v.type === "select") {
                                switch (v.label) {
                                    case "param_key":
                                        let paramKeys = [], ps = state.apiResults.params[state.paramSpace]
                                        if (ps) {
                                            paramKeys = Object.keys(ps)
                                        }
                                        return (
                                            <InputGroup style={{display: show}} key={i} className="mb-3" size="lg">
                                                <Form.Select size="lg" aria-label={v.label}>
                                                    <option>param key</option>
                                                    {paramKeys.map((v, i) => (<option key={i} value={v}>{v}</option>))}
                                                </Form.Select></InputGroup>
                                        )
                                    case "param_space":
                                        return (
                                            <InputGroup
                                                style={{display: show}} key={i} className="mb-3" size="lg">
                                                <Form.Select size="lg" onChange={onChange} aria-label={v.label}>
                                                    <option>param space</option>
                                                    <option value="Consensus">consensus</option>
                                                    <option value="Validator">validator</option>
                                                    <option value="Governance">governance</option>
                                                    <option value="Fee">fee</option>
                                                </Form.Select>
                                            </InputGroup>
                                        )
                                }
                            }
                            return (
                                <InputGroup style={{display: show}} key={i} className="mb-3" size="lg">
                                    {withTooltip(
                                        <InputGroup.Text className="param-input">
                                            {v.inputText}
                                        </InputGroup.Text>, v.tooltip, i)}
                                    <Form.Control placeholder={v.placeholder} required={v.required} defaultValue={v.defaultValue}
                                                  type={v.type} min={0} minLength={v.minLength} maxLength={v.maxLength} aria-label={v.label}
                                    />
                                </InputGroup>
                            )
                        })}
                        {renderJSONViewer()}
                    </Modal.Body>
                    <Modal.Footer>
                        {renderButtons()}
                    </Modal.Footer>
                </Form>
            </Modal>
        </div>
    </>
}