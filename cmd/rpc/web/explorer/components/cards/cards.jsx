import {Card, Col, Row} from "react-bootstrap";
import Truncate from 'react-truncate-inside';
import {addDate, formatBytes, formatNumber, formatTime} from "@/components/util";

const cardImages = ["./block-filled.png", "./chart-up.png", "./transaction-filled.png", "./lock-filled.png"]
const cardTitles = ["Latest Block", "Supply", "Transactions", "Validators"]

function getCardInfo1(props, idx) {
    const blks = props.blocks
    switch (idx) {
        case 0:
            return formatNumber(blks.results[0].block_header.height)
        case 1:
            return formatNumber(props.supply.total)
        case 2:
            if (blks.results[0].block_header.num_txs == null) {
                return "+0"
            }
            return "+" + formatNumber(blks.results[0].block_header.num_txs)
        case 3:
            let totalStake = 0
            // props.consVals.results.forEach(function (validator) { TODO fix
            //     totalStake += Number(validator.voting_power)
            // })
            return <>{formatNumber(totalStake)}<span style={{fontSize: "14px"}}>{" stake"}</span></>
    }
}

function getCardInfo2(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            return formatTime(v.results[0].block_header.time)
        case 1:
            return formatNumber(Number(props.supply.total) - Number(props.supply.staked), 1000) + " liquid"
        case 2:
            return "blk size: " + formatBytes(v.results[0].meta.size)
        case 3:
            // return props.consVals.results.length + " unique vals" TODO fix
            return 1 + " unique vals"
    }
}

function getCardInfo3(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            return v.results[0].meta.took
        case 1:
            return formatNumber(props.supply.staked, 1000) + " staked"
        case 2:
            return "block #" + v.results[0].block_header.height
        case 3:
            return "threshold " + formatNumber(props.params.Validator.validator_min_stake, 1000)
    }
}

function getCardInfo4(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            return <div style={{height: "25px", paddingTop: "10px"}}><Truncate text={v.results[0].block_header.hash}/>
            </div>
        case 1:
            return "+" + Number(props.params.Validator.validator_block_reward) / 1000000 + "/blk"
        case 2:
            return "TOTAL " + formatNumber(v.results[0].block_header.total_txs)
        case 3:
            // return "MaxStake: " + formatNumber(props.consVals.results[0].voting_power, 1000) TODO fix
            return "MaxStake: " + formatNumber(1000000, 1000)
        default:
            return "?"
    }
}


function getCardInfo5(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            return "Next block: " + addDate(v.results[0].block_header.time, v.results[0].meta.took)
        case 1:
            let s = "DAO pool supply: "
            if (props.pool != null) {
                return s + formatNumber(props.pool.amount, 1000)
            }
            return s
        case 2:
            let totalFee = 0, txs = v.results[0].transactions;
            if (txs == null || txs.length === 0) {
                return "Average fee in last blk: 0"
            }
            txs.forEach(function (tx) {
                totalFee += Number(tx.transaction.fee)
            })
            return "Average fee in last blk: " + formatNumber(totalFee / txs.length, 1000000)
        case 3:
            let totalStake = 0
            // props.consVals.results.forEach(function (validator) { TODO fix
            //     totalStake += Number(validator.voting_power)
            // })
            return (totalStake / props.supply.staked * 100).toFixed(1) + "% in consensus"
    }
}

function getOnClick(props, index) {
    if (index === 0) {
        return () => props.openModal(0)
    } else {
        if (index === 1) {
            return () => props.selectTable(3, 0)
        } else if (index === 2) {
            return () => props.selectTable(1, 0)
        }
        return () => props.selectTable(index + 1, 0)
    }
}

export default function Cards(props) {
    const cardData = props.state.cardData
    return (<Row sm={1} md={2} lg={4} className="g-4">
        {Array.from({length: 4}, (_, idx) => {
            return <Col key={idx}>
                <Card className="text-center">
                    <Card.Body className="card-body" onClick={getOnClick(props, idx)}>
                        <div className="card-image" style={{backgroundImage: "url(" + cardImages[idx] + ")"}}/>
                        <Card.Title className="card-title">{cardTitles[idx]}</Card.Title>
                        <h5>{getCardInfo1(cardData, idx)}</h5>
                        <Card.Text>
                            <span>{getCardInfo2(cardData, idx)}</span>
                            <span className="card-info-3">{getCardInfo3(cardData, idx)}</span>
                            <br/>
                            <span className="card-info-4">{getCardInfo4(cardData, idx)}</span>
                        </Card.Text>
                        <Card.Footer className="card-footer">{getCardInfo5(cardData, idx)}</Card.Footer>
                    </Card.Body>
                </Card>
            </Col>
        })}
    </Row>);
}