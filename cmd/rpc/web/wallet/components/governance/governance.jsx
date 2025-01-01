import {useEffect, useState} from "react";
import Truncate from 'react-truncate-inside';
import Container from "react-bootstrap/Container";
import JsonView from "@uiw/react-json-view";
import Form from 'react-bootstrap/Form';
import {Accordion, Button, Carousel, Col, InputGroup, Modal, Row, Spinner, Table} from "react-bootstrap";
import {AddVote, DelVote, Params, Poll, Proposals, RawTx, StartPoll, TxChangeParameter, TxDAOTransfer, VotePoll} from "@/components/api";
import {copy, getFormInputs, objEmpty, onFormSubmit, placeholders, renderToast, withTooltip} from "@/components/util";
import React from 'react';
import {Bar} from 'react-chartjs-2';
import {
    Chart as ChartJS,
    BarElement,
    CategoryScale,
    LinearScale,
    Tooltip,
    Legend,
} from 'chart.js';

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend);

export default function Governance({account: accountWithTxs}) {
    const [state, setState] = useState({
        txResult: {}, rawTx: {}, showPropModal: false, apiResults: {}, paramSpace: "", voteOnPollAccord: "1", voteOnProposalAccord: "1",
        propAccord: "1", txPropType: 0, toast: "", voteJSON: {}, pwd: "",
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
    } else {
        state.apiResults.poll["PLACEHOLDER EXAMPLE"] = placeholders.poll["PLACEHOLDER EXAMPLE"]
    }
    if (objEmpty(state.apiResults.proposals)) {
        state.apiResults.proposals = placeholders.proposals
    }
    const addVoteAPI = (json, approve) => AddVote(json, approve).then(_ => setState({...state, voteOnProposalAccord: "1", toast: "Voted!"}))
    const delVoteAPI = (json) => DelVote(json).then(_ => setState({...state, voteOnProposalAccord: "1", toast: "Deleted!"}))
    const startPollAPI = (address, json, password) => StartPoll(address, json, password).then(_ => setState({...state, voteOnPollAccord: "1", toast: "Started Poll!"}))
    const votePollAPI = (address, json, approve, password) => VotePoll(address, json, approve, password).then(_ => setState({...state, voteOnPollAccord: "1", toast: "Voted!"}))
    const handleClose = () => setState({...state, paramSpace: "", txResult: {}, showPropModal: false})
    const handlePropOpen = (type) => setState({...state, txPropType: type, showPropModal: true, paramSpace: "", txResult: {}})
    const sendRawTx = (j) => RawTx(j).then(r => {
        copy(state, setState, r, "tx hash copied to keyboard!")
    })
    const createDAOTransferTx = (address, amount, startBlock, endBlock, memo, fee, password) => {
        TxDAOTransfer(address, amount, startBlock, endBlock, memo, fee, password, false).then(res => {
            setState({...state, txResult: res})
        })
    }
    const createParamChangeTx = (address, paramSpace, paramKey, paramValue, startBlock, endBlock, memo, fee, password) => {
        TxChangeParameter(address, paramSpace, paramKey, paramValue, startBlock, endBlock, memo, fee, password, false).then(res => {
            setState({...state, txResult: res})
        })
    }
    const onPropSubmit = (e) => onFormSubmit(state, e, (r) => {
        if (state.txPropType === 0) {
            createParamChangeTx(r.sender, r.param_space, r.param_key, r.param_value, r.start_block, r.end_block, r.memo, r.fee, r.password)
        } else {
            createDAOTransferTx(r.sender, r.amount, r.start_block, r.end_block, r.memo, r.fee, r.password)
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
    const renderHeader = (title, img) => (
        <>
            <img className="governance-header-image" alt="vote" src={img}/>
            <span id="propose-title">{title}</span><span id="propose-subtitle">  on CANOPY</span>
            <br/><br/>
            <hr className="gov-header-hr"/>
            <br/><br/>
        </>
    )
    const renderAccord = (title, keyName, targetName, buttons, showPwd, placeholder = placeholders.params) => {
        let s = {...state}, activeKey = state[keyName], ph = JSON.stringify(placeholder, null, "  ")
        const onSelect = (i) => {
            s[keyName] = i
            setState(s)
        }
        const onChange = (e) => {
            s[targetName] = e.target.value
            setState({...s, voteJSON: e.target.value})
        }
        const onPwdChange = (e) => {
            setState({...s, pwd: e.target.value})
        }
        return <Accordion className="accord" activeKey={activeKey} onSelect={onSelect}>
            <Accordion.Item className="accord-item" eventKey="0">
                <Accordion.Header>{title}</Accordion.Header>
                <Accordion.Body>
                    <Form.Control className="accord-body-container" defaultValue={ph} as="textarea" onChange={onChange}/>
                    <br/>
                    <InputGroup style={{display: showPwd ? "" : "none"}} className="accord-pass-container" size="lg">
                        <InputGroup.Text>Password</InputGroup.Text>
                        <Form.Control onChange={onPwdChange} required={true} type={"password"}/>
                    </InputGroup>
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
                    return <Carousel.Item key={idx}>
                        <h6 className="poll-prop-hash">{val.proposalHash}</h6>
                        <a href={val.proposalURL} className="poll-prop-url">{val.proposalURL}</a>
                        <Container className="poll-carousel-container" fluid>
                            <Bar data={{
                                labels: [val.accounts.votedPercent + '% Accounts Reporting', val.validators.votedPercent + '% Validators Reporting'], // Categories
                                datasets: [
                                    {
                                        label: '% Voted YES',
                                        data: [val.accounts.approvedPercent, val.validators.approvedPercent],
                                        backgroundColor: '#7749c0',
                                    },
                                    {
                                        label: '% Voted NO',
                                        data: [val.accounts.rejectPercent, val.validators.rejectPercent],
                                        backgroundColor: '#000',
                                    },
                                ],
                            }} options={{
                                responsive: true,
                                plugins: {
                                    tooltip: {
                                        enabled: true, // Enable tooltips for more detail
                                    },
                                },
                                scales: {
                                    y: {
                                        beginAtZero: true,
                                        max: 100, // Percentage scale
                                    },
                                },
                            }}/>
                            <br/>
                        </Container>
                    </Carousel.Item>
                })}
            </Carousel>
            <br/>
            {renderAccord("START OR VOTE ON POLL", "voteOnPollAccord", "voteJSON", [
                {title: "START NEW", onClick: () => startPollAPI(accountWithTxs.account.address, state.voteJSON, state.pwd)},
                {title: "APPROVE", onClick: () => votePollAPI(accountWithTxs.account.address, state.voteJSON, true, state.pwd)},
                {title: "REJECT", onClick: () => votePollAPI(accountWithTxs.account.address, state.voteJSON, false, state.pwd)},
            ], true, placeholders.pollJSON)}
            {renderHeader("propose", "./proposal.png")}
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
            {renderAccord("VOTE ON PROPOSAL", "voteOnProposalAccord", "voteJSON", [
                {title: "APPROVE", onClick: () => addVoteAPI(state.voteJSON, true)},
                {title: "REJECT", onClick: () => addVoteAPI(state.voteJSON, false)},
                {title: "DELETE", onClick: () => delVoteAPI(state.voteJSON)},
            ], false)}
            {renderToast(state, setState)}
            <br/>
            {renderAccord("EXISTING PROPOSAL", "propAccord", "rawTx", [
                {title: "SUBMIT", onClick: () => sendRawTx(state.rawTx)}
            ], false, placeholders.rawTx)}
            <Button className="propose-button" onClick={() => handlePropOpen(0)} variant="outline-dark">New Protocol Change</Button>
            <Button className="propose-button" onClick={() => handlePropOpen(1)} variant="outline-dark">New Treasury Subsidy</Button>
            <br/><br/>
            <Modal show={state.showPropModal} size="lg" onHide={handleClose}>
                <Form onSubmit={onPropSubmit}>
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