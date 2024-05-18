import React, {useState} from 'react';
import {
    MDBCollapse,
    MDBRipple,
    MDBListGroup
} from 'mdb-react-ui-kit';

import OverlayTrigger from 'react-bootstrap/OverlayTrigger';
import Tooltip from 'react-bootstrap/Tooltip';

const sidebarIcons = [
    "./block.png", "./transaction2.png", "./pending2.png", "./account.png", "./validator.png", "./gov.png"
]
const toolTip = [
    "Blocks", "Transactions", "Pending", "Accounts", "Validators", "Governance", "Explore"
]



export default function Sidebar({selectTable}) {
    return (
        <>
            <MDBCollapse tag="nav" style={{backgroundColor: "#f1f5f9", width: "5%"}}
                         className="d-lg-block sidebar">
                <div className="position-sticky">
                    <MDBListGroup className="mx-4 mt-4">
                        {sidebarIcons.map((k, i) => (
                            <OverlayTrigger
                                key={i}
                                placement="right"
                                delay={{show: 250, hide: 400}}
                                overlay={<Tooltip id="button-tooltip">{toolTip[i]}</Tooltip>}
                            >
                                <MDBRipple onClick={() => selectTable(i, 0)} rippleTag='span' className="sidebar-icon-container">
                                    <a href="#">
                                        <div className="sidebar-icon" style={{backgroundImage: "url(" + k + ")"}}/>
                                    </a>
                                </MDBRipple>
                            </OverlayTrigger>
                        ))}
                        <OverlayTrigger
                            placement="right"
                            delay={{show: 250, hide: 400}}
                            overlay={<Tooltip id="button-tooltip">{"Explore"}</Tooltip>}
                        >
                            <MDBRipple rippleTag='span' className="sidebar-icon-container">
                                <a href="https://docs.google.com">
                                    <div className="sidebar-icon" style={{backgroundImage: "url(./explore.png)"}}/>
                                </a>
                            </MDBRipple>
                        </OverlayTrigger>
                    </MDBListGroup>
                </div>
                <a href="https://github.com">
                <div id="sidebar-social" style={{
                    backgroundImage: "url(./github.png)",
                    backgroundSize: "50%",
                    backgroundRepeat: "no-repeat",
                    backgroundPosition: "center",
                    position: "absolute",
                    bottom: "0",
                    right: "1",
                    left: "1",
                    margin: "0 auto",
                    width: "100%",
                    height: "50px",
                    paddingBottom: "100px"
                }} className="justify-content-end"/>
                </a>
            </MDBCollapse>
        </>
    );
}