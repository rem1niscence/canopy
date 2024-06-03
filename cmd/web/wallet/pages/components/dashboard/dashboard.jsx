import {Button, Card, Carousel, Col, Row, Table} from "react-bootstrap";
import Container from "react-bootstrap/Container";
import react, {useEffect, useState} from "react";
import dynamic from "next/dynamic";
import {
    adminRPCURL,
    configPath,
    ConsensusInfo,
    consensusInfoPath,
    Logs, logsPath, peerBookPath, PeerInfo, peerInfoPath,
    Resource,
} from "@/pages/components/api";
import Truncate from 'react-truncate-inside';
import {
    YAxis,
    Tooltip,
    Legend,
    AreaChart,
    Area,
} from 'recharts';


const LazyLog = dynamic(() => import('react-lazylog').then((mod) => mod.LazyLog), {
    ssr: false,
});


function numberWithCommas(x) {
    return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function formatNumber(nString, cutoff) {
    if (nString == null) {
        return "zero"
    }
    if (Number(nString) < cutoff) {
        return numberWithCommas(nString)
    }
    return Intl.NumberFormat("en", {notation: "compact", maximumSignificantDigits: 8}).format(nString)
}

function getRatio(a, b) {
    if (a > b) {
        var bg = a;
        var sm = b;
    } else {
        var bg = b;
        var sm = a;
    }
    for (var i = 1; i < 1000000; i++) {
        var d = sm / i;
        var res = bg / d;
        var howClose = Math.abs(res - res.toFixed(0));
        if (howClose < .1) {
            if (a > b) {
                return res.toFixed(0) + ':' + i;
            } else {
                return i + ':' + res.toFixed(0);
            }
        }
    }
}

function getRoundProgress(consensusInfo) {
    switch (consensusInfo.view.phase){
        case "ELECTION":
            return Number(0/8*100)
        case "ELECTION-VOTE":
            return Number(1/8*100)
        case "PROPOSE":
            return Number(2/8*100)
        case "PROPOSE-VOTE":
            return Number(3/8*100)
        case "PRECOMMIT":
            return Number(4/8*100)
        case "PRECOMMIT-VOTE":
            return Number(5/8*100)
        case "COMMIT":
            return Number(6/8*100)
        case "COMMIT-PROCESS":
            return Number(7/8*100)
    }
    return 100
}

function Dashboard() {
    const [logs, setLogs] = useState("retrieving logs...")
    const [pauseLogs, setPauseLogs] = useState(false)
    const [resource, setResource] = useState([])
    const [consensusInfo, setConsensusInfo] = useState({})
    const [peerInfo, setPeerInfo] = useState({})
    async function queryAPI() {
        await ConsensusInfo().then(res => {
            setConsensusInfo(res)
        })
        await PeerInfo().then(res => {
            setPeerInfo(res)
        })
        await Resource().then(res => {
            if (resource.length >= 30) {
                let [first, ...nextResource] = resource;
                setResource([...nextResource, res])
            } else {
                setResource([...resource, res])
            }
        })
    }
    useEffect(() => {
        const interval = setInterval(() => {
            if (!pauseLogs) {
                Logs().then(res => {
                    setLogs(res.toString())
                })
            }
            queryAPI()
        }, 1000);
        return () => clearInterval(interval);
    });

    let text = logs.replace("\n", '');

    if (consensusInfo.view == null || peerInfo.id == null) {
        queryAPI()
        return <>Loading...</>
    }
    let inboundCount = Number(peerInfo.numInbound)?Number(peerInfo.numInbound):0
    let outboundCount = Number(peerInfo.numOutbound)?Number(peerInfo.numOutbound):0
    let ioPeer = "INBOUND: "+ inboundCount + ", OUTBOUND: " + outboundCount
    let ioRatio = getRatio(inboundCount, outboundCount)
    return <>
        <div className="content-container" style={{backgroundColor: "#000", mixBlendMode: "difference"}}>
            <Container style={{marginBottom: "20px"}} fluid>
                <Row>
                    <Col>
                        <Carousel slide={false} interval={null} className="carousel">
                            <Carousel.Item>
                                <Card className="carousel-item-container"
                                      style={{backgroundColor: "black", color: "white"}}>
                                    <Card.Body>
                                        <Card.Title style={{fontSize: "18px", fontWeight: "800"}}><span
                                            style={{color: "white"}}>{consensusInfo.syncing?"SYNCING":"SYNCED"}</span></Card.Title>
                                        <div style={{fontWeight: "bold", fontSize: "14px"}}>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                color: "yellowgreen",
                                                marginBottom: "10px",
                                                textAlign: "left"
                                            }}>{"H: " + formatNumber(consensusInfo.view.height, 1000000000)+", R: "+consensusInfo.view.round+", P: " + consensusInfo.view.phase}</p>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                marginBottom: "10px",
                                                textAlign: "left",
                                                fontWeight: "400",
                                                color: "#7b7b7b"
                                            }}><Truncate text={consensusInfo.proposer===""?"PROP: UNDECIDED":"PROP: " + consensusInfo.proposer}/></p>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                marginBottom: "10px",
                                                textAlign: "left",
                                                fontWeight: "400",
                                                color: "#7b7b7b"
                                            }}><Truncate
                                                text={consensusInfo.blockHash===""?"BLK: WAITING":"BLK: " + consensusInfo.blockHash}/>
                                            </p>
                                            <p>{consensusInfo.status}</p>
                                        </div>
                                    </Card.Body>
                                </Card>
                            </Carousel.Item>
                            <Carousel.Item>
                                <Card className="carousel-item-container"
                                      style={{backgroundColor: "black", color: "white"}}>
                                    <Card.Body>
                                        <Card.Title
                                            style={{fontSize: "18px", fontWeight: ""}}><span>{"ROUND PROGRESS: "+ getRoundProgress(consensusInfo) +"%"}</span></Card.Title>
                                        <div style={{fontWeight: "bold", fontSize: "14px"}}>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                marginBottom: "10px",
                                                fontWeight: "normal",
                                                color: "yellowgreen"
                                            }}><Truncate text={"ADDRESS: "+consensusInfo.address}/></p>
                                            {/*<p style={{fontWeight: "bold"}}><Truncate text="VP: 670000/1000000"/></p>*/}
                                        </div>
                                    </Card.Body>
                                </Card>
                            </Carousel.Item>
                            <Carousel.Item>
                                <Card className="carousel-item-container"
                                      style={{backgroundColor: "black", color: "white"}}>
                                    <Card.Body>
                                        <Card.Title style={{fontSize: "18px", fontWeight: "300"}}><span
                                            style={{color: ""}}>EXPLORE RAW JSON</span></Card.Title>
                                        <div style={{fontWeight: "bold", fontSize: "14px",}}>
                                            <Button style={{margin: "5"}} onClick={()=>window.open(adminRPCURL+consensusInfoPath, "_blank", "noreferrer")} variant="outline-secondary">QUORUM</Button>
                                            <Button style={{margin: "5"}} onClick={()=>window.open(adminRPCURL+configPath, "_blank", "noreferrer")} variant="outline-secondary">CONFIG</Button>
                                            <Button style={{margin: "5"}} onClick={()=>window.open(adminRPCURL+logsPath, "_blank", "noreferrer")} variant="outline-secondary">LOGGER</Button>
                                            <br/><br/>
                                        </div>
                                    </Card.Body>
                                </Card>
                            </Carousel.Item>
                        </Carousel>
                    </Col>
                    <Col>
                        <Carousel slide={false} interval={null} className="carousel">
                            <Carousel.Item>
                                <Card className="carousel-item-container"
                                      style={{backgroundColor: "black", color: "white"}}>
                                    <Card.Body>
                                        <div style={{fontWeight: "bold", fontSize: "14px"}}>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                fontSize: "18px",
                                                fontWeight: "800",
                                                marginBottom: "10px",
                                                color: "white"
                                            }}>{peerInfo.numPeers == null?"TOTAL PEERS: 0":"TOTAL PEERS: " +peerInfo.numPeers}</p>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                color: "yellowgreen",
                                                marginBottom: "10px",
                                            }}>{ioPeer}</p>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                marginBottom: "10px",
                                                textAlign: "left",
                                                fontWeight: "400",
                                                color: "#7b7b7b"
                                            }}><Truncate
                                                text={"ID: " + peerInfo.id.public_key}/>
                                            </p>
                                            <p style={{
                                                width: "50%",
                                                margin: "0 auto",
                                                marginBottom: "10px",
                                                fontWeight: "400",
                                                color: "#7b7b7b"
                                            }}><Truncate
                                                text={peerInfo.id.net_address?"NET ADDR: "+ peerInfo.id.net_address:"NET ADDR: External Address Not Set"}/>
                                            </p>
                                            <p style={{fontWeight: "600"}}>{ioRatio?"I / O RATIO "+ ioRatio: "I / O RATIO 0:0"}</p>
                                        </div>
                                    </Card.Body>
                                </Card>
                            </Carousel.Item><Carousel.Item>
                            <Card className="carousel-item-container"
                                  style={{backgroundColor: "black", color: "white"}}>
                                <Card.Body>
                                    <Card.Title style={{fontSize: "18px", fontWeight: "300"}}><span
                                        style={{color: ""}}>EXPLORE RAW JSON</span></Card.Title>
                                    <div style={{fontWeight: "bold", fontSize: "14px",}}>
                                        <Button style={{margin: "5"}} onClick={()=>window.open(adminRPCURL+peerBookPath, "_blank", "noreferrer")}variant="outline-secondary">PEER BOOK</Button>
                                        <Button style={{margin: "5"}} onClick={()=>window.open(adminRPCURL+peerInfoPath, "_blank", "noreferrer")}variant="outline-secondary">PEER INFO</Button>
                                        <br/><br/>
                                    </div>
                                </Card.Body>
                            </Card>
                        </Carousel.Item>

                        </Carousel>
                    </Col>
                </Row>
            </Container>
            <hr style={{
                border: "1px dashed white",
                marginBottom: "10px",
                borderRadius: "5px",
                width: "25%",
                margin: "0 auto"
            }}/>
            <div onClick={() => setPauseLogs(!pauseLogs)} className="logs-button-container">
                {!pauseLogs ? <img className="logs-button" alt="pause" src="./pause_filled.png"/> :
                    <img className="logs-button" alt="play" src="./unpause_filled.png"/>}
            </div>
            <LazyLog enableSearch={true} style={{
                textAlign: "left",
                height: "300px",
                marginBottom: "50px",
                border: "2px solid #222222",
                backgroundColor: "#000",
                filter: "grayscale(100%)"
            }} text={text}/>
            <div style={{height: "350px", width: "100%"}}/>
            <Container style={{marginBottom: "50px"}}>
                <Row>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{border: "2px solid #222222", borderRadius: "6px", margin: "0 auto"}}
                                   margin={{top: 40, right: 40, left: 20, bottom: 20}}
                        >
                            <YAxis tickCount={1} label={{value: "PROCESS", angle: -90}}/>
                            <Area name="CPU %" type="monotone" dataKey="process.usedCPUPercent" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#ram)"/>
                            <Area name="RAM %" type="monotone" dataKey="process.usedMemoryPercent" stroke="#848484"
                                  fillOpacity={1} fill="url(#cpu)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{border: "2px solid #222222", borderRadius: "6px", margin: "0 auto"}}
                                   margin={{top: 30, right: 30, left: 0, bottom: 0}}>
                            <YAxis tickCount={1} label={{value: "SYSTEM", angle: -90}}/>
                            <Area name="CPU %" type="monotone" dataKey="system.usedCPUPercent" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#cpu)"/>
                            <Area name="RAM %" type="monotone" dataKey="system.usedRAMPercent" stroke="#848484"
                                  fillOpacity={1} fill="url(#ram)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                </Row>
                <Row>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{
                                       border: "2px solid #222222",
                                       borderTop: "none",
                                       borderRadius: "6px",
                                       margin: "0 auto"
                                   }}
                                   margin={{top: 30, right: 30, left: 0, bottom: 0}}>
                            <YAxis tickCount={1} label={{value: "DISK", angle: -90}}/>
                            <Area name="Disk %" type="monotone" dataKey="system.usedDiskPercent" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#cpu)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{
                                       border: "2px solid #222222",
                                       borderTop: "none",
                                       borderRadius: "6px",
                                       margin: "0 auto"
                                   }}
                                   margin={{top: 30, right: 30, left: 0, bottom: 0}}>
                            <YAxis tick={false} label={{value: "IN OUT", angle: -90}}/>
                            <Area name="Received" type="monotone" dataKey="system.ReceivedBytesIO" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#ram)"/>
                            <Area name="Written" type="monotone" dataKey="system.WrittenBytesIO" stroke="#848484"
                                  fillOpacity={1} fill="url(#cpu)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                </Row>
                <Row>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{
                                       border: "2px solid #222222",
                                       borderTop: "none",
                                       borderRadius: "6px",
                                       margin: "0 auto"
                                   }}
                                   margin={{top: 30, right: 30, left: 0, bottom: 0}}>
                            <YAxis tickCount={1} label={{value: "IN OUT", angle: -90}}/>
                            <Area name="Received" type="monotone" dataKey="process.threadCount" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#ram)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                    <Col>
                        <AreaChart width={600} height={250} data={resource}
                                   style={{
                                       border: "2px solid #222222",
                                       borderTop: "none",
                                       borderRadius: "6px",
                                       margin: "0 auto"
                                   }}
                                   margin={{top: 30, right: 30, left: 0, bottom: 0}}>
                            <YAxis tickCount={1} label={{value: "PROCESS", angle: -90}}/>
                            <Area name="File Descriptors" type="monotone" dataKey="process.fdCount" stroke="#eeeeee"
                                  fillOpacity={1} fill="url(#cpu)"/>
                            <Tooltip contentStyle={{backgroundColor: "#222222"}}/>
                            <Legend/>
                        </AreaChart>
                    </Col>
                </Row>
            </Container>
            <div style={{height: "50px", width: "100%"}}/>
        </div>

    </>
}

export default Dashboard;