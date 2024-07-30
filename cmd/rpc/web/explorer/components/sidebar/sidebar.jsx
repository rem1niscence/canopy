import React from 'react';
import {MDBCollapse, MDBRipple, MDBListGroup} from 'mdb-react-ui-kit';
import {withTooltip} from "@/components/util";

const sidebarIcons = ["./block.png", "./transaction.png", "./pending2.png", "./account.png", "./validator.png", "./gov.png"]
const toolTip = ["Blocks", "Transactions", "Pending", "Accounts", "Validators", "Governance", "Explore"]
const urls = {docs: "https://docs.google.com", github: "https://github.com"}

export default function Sidebar({selectTable}) {
    return (
        <>
            <MDBCollapse className="d-lg-block sidebar">
                <MDBListGroup className="sidebar-list">
                    {sidebarIcons.map((k, i) => {
                        return withTooltip(
                            <MDBRipple onClick={() => selectTable(i, 0)} className="sidebar-icon-container">
                                <div className="sidebar-icon" style={{backgroundImage: "url(" + k + ")"}}/>
                            </MDBRipple>, toolTip[i], i)
                    })}
                    {
                        withTooltip(<MDBRipple className="sidebar-icon-container">
                            <a href={urls.docs}>
                                <div className="sidebar-icon" style={{backgroundImage: "url(./explore.png)"}}/>
                            </a>
                        </MDBRipple>, "Explore")
                    }
                </MDBListGroup>
                <a href={urls.github}>
                    <div id="sidebar-social" style={{backgroundImage: "url(./github.png)"}}/>
                </a>
            </MDBCollapse>
        </>
    );
}