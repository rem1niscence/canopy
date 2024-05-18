import {Card, CardGroup, Col, Modal, OverlayTrigger, Row, Tab, Tabs, Tooltip} from "react-bootstrap";

const cardImages = [
    "./block-filled.png",
    "./chart-up.png",
    "./transaction-filled.png",
    "./lock-filled.png"
]

const cardTitles = [
    "Latest Block",
    "Supply",
    "Transactions",
    "Validators"
]

function numberWithCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function formatBytes(a, b = 2) {
    if (!+a) return "0 Bytes";
    const c = 0 > b ? 0 : b, d = Math.floor(Math.log(a) / Math.log(1024));
    return `${parseFloat((a / Math.pow(1024, d)).toFixed(c))} ${["B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"][d]}`
}


Date.prototype.addMS = function (s) {
    this.setTime(this.getTime() + (s));
    return this;
}

function string_to_milliseconds(str, take_sum = false) {
    const units = {
        yr: {re: /(\d+|\d*\.\d+)\s*(Y)/i, s: 1000 * 60 * 60 * 24 * 365}, // Case insensitive "Y"
        mo: {re: /(\d+|\d*\.\d+)\s*(M|[Mm][Oo])/, s: 1000 * 60 * 60 * 24 * 30}, // Case insensitive "mo" or sensitive "M"
        wk: {re: /(\d+|\d*\.\d+)\s*(W)/i, s: 1000 * 60 * 60 * 24 * 7}, // Case insensitive "W"
        dy: {re: /(\d+|\d*\.\d+)\s*(D)/i, s: 1000 * 60 * 60 * 24}, // Case insensitive "D"
        hr: {re: /(\d+|\d*\.\d+)\s*(h)/i, s: 1000 * 60 * 60}, // Case insensitive "h"
        mn: {re: /(\d+|\d*\.\d+)\s*(m(?![so])|[Mm][Ii]?[Nn])/, s: 1000 * 60}, // Case insensitive "min"/"mn" or sensitive "m" (if not followed by "s" or "o")
        sc: {re: /(\d+|\d*\.\d+)\s*(s)/i, s: 1000}, // Case insensitive "s"
        ms: {re: /(\d+|\d*\.\d+)\s*(ms|mil)/i, s: 1}, // Case insensitive "ms"/"mil"
    };

    return Object.values(units).reduce((sum, unit) => sum + (str.match(unit.re)?.[1] * unit.s || 0), 0);
}

function formatTime(value) {
    let d = new Date(Date.parse(value+" UTC"))
    return d.toLocaleTimeString()
}

function addDate(date, duration) {
    let ms = string_to_milliseconds(duration)
    let d = new Date(Date.parse(date + " UTC"))
    return d.addMS(ms).toLocaleTimeString()
}

function formatNumber(nString, cutoff) {
    if (Number(nString) < cutoff) {
        return numberWithCommas(nString)
    }
    return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 3}).format(nString)
}

function truncate(str, n) {
    return (str.length > n) ? str.slice(0, n - 1) : str;
}

function getCardInfo1(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return formatNumber(v.results[0].block_header.height, 1000000)
        case 1:
            if (props.supply == null) {
                return "?"
            }
            return formatNumber(props.supply.total, 10000000000)
        case 2:
            if (v == null || v.results == null || v.results.length === 0) {
                return ["?"]
            }
            if (v.results[0].block_header.num_txs == null) {
                return "+0"
            }
            return "+" + formatNumber(v.results[0].block_header.num_txs, 1000000)
        case 3:
            let vals = props.consVals
            if (vals ==null || vals.results == null || vals.results.length === 0) {
                return ["?"]
            }
            let totalStake = 0
            vals.results.forEach(function (validator) {
                totalStake += Number(validator.voting_power)
            })
            return "\xa0" + formatNumber(totalStake, 1000000) + " stake"
        default:
            return "?"
    }
}

function getCardInfo2(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return formatTime(v.results[0].block_header.time)
        case 1:
            if (props.supply == null) {
                return "?"
            }
            let liquid = Number(props.supply.total) - Number(props.supply.staked)
            return formatNumber(liquid, 1000) + " liquid"
        case 2:
            if (v == null || v.results == null || v.results.length === 0) {
                return ["?"]
            }
            return "blk size: " + formatBytes(v.results[0].meta.size)
        case 3:
            let vals = props.consVals
            if (vals ==null || vals.results == null || vals.results.length === 0) {
                return ["?"]
            }
            let count = 0
            vals.results.forEach(function (_) {
                count += 1
            })
            return count + " unique vals"
        default:
            return "?"
    }
}

function getCardInfo3(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return v.results[0].meta.took
        case 1:
            if (props.supply == null) {
                return "?"
            }
            return formatNumber(props.supply.staked, 1000) + " staked"
        case 2:
            if (v == null || v.results == null || v.results.length === 0) {
                return ["?"]
            }
            return "block #" + v.results[0].block_header.height
        case 3:
            let vals = props.consVals
            if (vals ==null || vals.results == null || vals.results.length === 0) {
                return ["?"]
            }
            let count = 0
            vals.results.forEach(function (_) {
                count += 1
            })
            return "threshold " + formatNumber(props.params.Validator.validator_min_stake, 1000)
        default:
            return "?"
    }
}

function getCardInfo4(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return truncate(v.results[0].block_header.hash, 35) + "..."
        case 1:
            return "+" + Number(props.params.Validator.validator_block_reward) / 1000000 + "/blk"
        case 2:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return "TOTAL " + formatNumber(v.results[0].block_header.total_txs, 1000000)
        case 3:
            let vals = props.consVals
            if (vals ==null || vals.results == null || vals.results.length === 0) {
                return ["?"]
            }
            return "Max Staker: " + formatNumber(vals.results[0].voting_power, 1000)
        default:
            return "?"
    }
}


function getCardInfo5(props, idx) {
    const v = props.blocks
    switch (idx) {
        case 0:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            return "Next block: " + addDate(v.results[0].block_header.time, v.results[0].meta.took)
        case 1:
            if (v == null || v.results == null || v.results.length === 0) {
                return "?"
            }
            let s = "DAO pool supply: "
            if (props.pool != null) {
                return s + formatNumber(props.pool.amount, 1000)
            }
            return s
        case 2:
            if (v == null || v.results == null || v.results.length === 0) {
                return ["?"]
            }
            let txs = v.results[0].transactions
            if (txs == null || txs.length === 0) {
                return "Average fee: 0"
            }
            let totalFee = 0
            let count = 0
            txs.forEach(function (tx) {
                count += 1
                totalFee += Number(tx.transaction.fee)
            })
            return "Average fee: " + formatNumber(totalFee / count, 1000000)
        case 3:
            let vals = props.consVals
            if (vals ==null || vals.results == null || vals.results.length === 0) {
                return ["?"]
            }
            let totalStake = 0
            vals.results.forEach(function (validator) {
                totalStake += Number(validator.voting_power)
            })
            return (totalStake / props.supply.staked * 100).toFixed(1) + "% of stake is from Validators"
        default:
            return "?"
    }
}

function getOnClick(props, index) {
    if (index === 0) {
        return () => props.handleOpen(0)
    } else {
        if (index === 1) {
            return () => props.selectTable(3, 0)
        } else if (index === 2) {
            return () => props.selectTable(1, 0)
        }
        return () => props.selectTable(index+1, 0)
    }
}

export default function Cards(props) {
    return (<Row sm={1} md={2} lg={4} className="g-4">
            {Array.from({length: 4}, (_, idx) => (
                <Col key={idx}>
                    <Card className="text-center">
                        <Card.Body onClick={getOnClick(props, idx)} style={{textAlign: "left"}}>
                            <a href="#" style={{color: "black", textDecoration: "none"}}>
                                <div style={{
                                    width: "25px",
                                    height: "25px",
                                    marginRight: "10px",
                                    backgroundSize: "cover",
                                    backgroundRepeat: "no-repeat",
                                    float: "left",
                                    backgroundImage: "url(" + cardImages[idx] + ")"
                                }}/>
                                <Card.Title style={{
                                    fontSize: "12px",
                                    paddingTop: "10px",
                                    fontWeight: "bold"
                                }}>{cardTitles[idx]}</Card.Title>
                                <h5>{getCardInfo1(props.data, idx)}</h5>
                                <Card.Text>
                                    <span>{getCardInfo2(props.data, idx)}</span>
                                    <span style={{
                                        float: "right",
                                        fontSize: "12px",
                                        paddingTop: "5px"
                                    }}>{getCardInfo3(props.data, idx)}</span>
                                    <br/>
                                    <span style={{
                                        fontSize: "9px",
                                        color: "#2A1A1F",
                                        fontWeight: "bold"
                                    }}>{getCardInfo4(props.data, idx)}</span>
                                </Card.Text>
                                <Card.Footer style={{fontSize: "12px"}}
                                             className="text-muted">{getCardInfo5(props.data, idx)}</Card.Footer>
                            </a>
                        </Card.Body>
                    </Card>
                </Col>
            ))}
        </Row>
    );
}