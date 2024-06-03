import Head from 'next/head'
import styles from '@/styles/Home.module.css'
import Navigation from "@/pages/components/navbar/navbar";
import {AccountWithTxs, Keystore, Validator} from "@/pages/components/api";
import {useEffect, useState} from "react";
import Accounts from "@/pages/components/account/account";
import Dashboard from "@/pages/components/dashboard/dashboard";
import Governance from "@/pages/components/governance/governance";

export default function Home() {
    const [contentIdx, setContentIdx] = useState(0)
    const [keystore, setKeystore] = useState(null)
    const [activeKey, setActiveKey] = useState(0)
    const [activeAccount, setActiveAccount] = useState({})
    const [activeValidator, setActiveValidator] = useState({})

    function queryAPI(activeKey) {
        Keystore().then(res => {
            setKeystore(res)
            AccountWithTxs(0, Object.keys(res)[activeKey], 0).then(res => setActiveAccount(res))
            Validator(0, Object.keys(res)[activeKey]).then(res => setActiveValidator(res))
        })
    }

    function handleSetActiveKey(activeKey) {
        queryAPI(activeKey)
        setActiveKey(activeKey)
    }

    function renderContent(index) {
        if (index === 0) {
            return <Accounts keygroup={Object.values(keystore)[activeKey]} accountWithTxs={activeAccount}
                             validator={activeValidator}/>
        } else if (index === 1) {
            return <Governance keygroup={Object.values(keystore)[activeKey]} accountWithTxs={activeAccount}/>
        } else {
            return <Dashboard/>
        }
    }

    useEffect(() => {
        const interval = setInterval(() => {
            queryAPI(activeKey)
        }, 4000);
        return () => clearInterval(interval);
    });

    if (keystore === null) {
        queryAPI(activeKey)
        return <>Loading...</>
    } else {
        return <>
            <Head>
                <title>Web Wallet</title>
                <meta name="description" content="web wallet"/>
                <meta name="viewport" content="width=device-width, initial-scale=1"/>
                <link rel="icon" href="/favicon.ico"/>
            </Head>
            <div className={styles.container}>
                <Navigation keystore={keystore} setActiveKey={handleSetActiveKey} activeKey={activeKey}
                            setContentIndex={setContentIdx}/>
                <div className={styles.pageContent}>
                    {renderContent(contentIdx)}
                </div>
            </div>
        </>
    }
}
