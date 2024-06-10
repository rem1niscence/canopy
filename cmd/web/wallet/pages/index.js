import Navigation from "@/pages/components/navbar/navbar";
import {AccountWithTxs, Keystore, Validator} from "@/pages/components/api";
import {useEffect, useState} from "react";
import Accounts from "@/pages/components/account/account";
import Dashboard from "@/pages/components/dashboard/dashboard";
import Governance from "@/pages/components/governance/governance";
import {Spinner} from "react-bootstrap";

export default function Home() {
    const [state, setState] = useState({navIdx: 0, keystore: null, keyIdx: 0, account: {}, validator: {}})
    const setNavIdx = (i) => setState({...state, navIdx: i})

    function queryAPI(i = state.keyIdx) {
        Keystore().then(ks => {
            Promise.all(
                [AccountWithTxs(0, Object.keys(ks)[i], 0), Validator(0, Object.keys(ks)[i])]
            ).then((r) => {
                setState({...state, keyIdx: i, keystore: ks, account: r[0], validator: r[1]})
            })
        })
    }

    useEffect(() => {
        const i = setInterval(() => {
            queryAPI()
        }, 4000)
        return () => clearInterval(i)
    })

    if (state.keystore === null) {
        queryAPI()
        return <Spinner id="spinner"/>
    }
    return <>
        <div id="container">
            <Navigation {...state} setActiveKey={queryAPI} setNavIdx={setNavIdx}/>
            <div id="pageContent">
                {(() => {
                    if (state.navIdx === 0) {
                        return <Accounts keygroup={Object.values(state.keystore)[state.keyIdx]} {...state} />
                    } else if (state.navIdx === 1) {
                        return <Governance {...state} />
                    } else {
                        return <Dashboard/>
                    }
                })()}
            </div>
        </div>
    </>
}
