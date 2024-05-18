import Head from 'next/head'
import styles from '@/styles/Home.module.css'
import Navigation from "@/pages/components/navbar/navbar";
import Sidebar from "@/pages/components/sidebar/sidebar";
import DataTable from "@/pages/components/table/table"
import DetailModal from "@/pages/components/modal/modal"
import Cards from "@/pages/components/cards/cards";
import {useEffect, useState} from "react";
import * as API from "@/pages/components/api";

async function getDataForTable(idx, page, setFocus, setIndex) {
    let data = {}
    switch (idx) {
        case 0:
            data = await API.Blocks(page, 0)
            break
        case 1:
            data = await API.Transactions(page, 0, focus)
            break
        case 2:
            data = await API.Pending(page, 0, focus)
            break
        case 3:
            data = await API.Accounts(page, 0, focus)
            break
        case 4:
            data = await API.Validators(page, 0, focus)
            break
        case 5:
            data = await API.Params(page, 0, focus)
            break
    }
    setFocus(data)
    setIndex(idx)
}

async function getCardData(index, page, setFocus, setIndex, setState) {
    let fullData = {}
    fullData.blocks = await API.Blocks(1, 0)
    fullData.consVals = await API.ConsValidators(1, 0)
    fullData.valdiators = await API.ConsValidators(1, 0)
    fullData.supply = await API.Supply(0, 0)
    fullData.pool = await API.DAO(0, 0)
    fullData.params = await API.Params(0, 0)
    await getDataForTable(index, page, setFocus, setIndex)
    setState({
        loading: false,
        cardData: fullData
    })
}

async function getModalData(data, page) {
    let noResult = "no result found"
    if (typeof data === "string") {
        if (data.length === 64) {
            let block = await API.BlockByHash(data)
            if (block.block_header == null || block.block_header.hash == null) {
                let tx = await API.TxByHash(data)
                if (tx == null || tx.sender == null) {
                    return noResult
                }
                return tx
            }
            return {"block": block}
        } else if (data.length === 40) {
            let val = await API.Validator(0, data)
            let acc = await API.AccountWithTxs(0, data, page)
            if (acc.account.address == null && val.address == null) {
                return noResult
            } else if (acc.account.address == null) {
                return {"validator": val}
            } else if (val.address == null) {
                return acc
            }
            acc.validator = val
            return acc
        }
        return noResult
    } else {
        let block = await API.BlockByHeight(data)
        if (block.block_header == null || block.block_header.hash == null) {
            return noResult
        }
        return {"block": block}
    }
}

export default function Home() {
    const [modalState, setModalState] = useState({
        show: false,
        page: 0,
        query: "",
        data: {}
    });
    const [index, setIndex] = useState(0)
    const [modalPage, setModalPage] = useState(1)
    const [focus, setFocus] = useState({})
    const [state, setState] = useState({
        loading: true,
        page: 0,
        cardData: {},
    });
    const handleClose = () => setModalState({show: false, query: "", page: 0, data: {}});

    async function handleOpen(p, page) {
        let data = await getModalData(p, page)
        setModalState({show: true, query: p, page: page, data: data})
    }

    function selectTable(index, page) {
        getDataForTable(index, page, setFocus, setIndex)
        setModalPage(1)
    }

    useEffect(() => {
        const interval = setInterval(() => {
            getCardData(index, 0, setFocus, setIndex, setState)
        }, 4000);
        return () => clearInterval(interval);
    });
    if (state.loading) {
        getCardData(index, 0, setFocus, setIndex, setState)
        return <div>Loading...</div>
    } else {
        return <>
            <Head>
                <title>Block Explorer</title>
                <meta name="description" content="block explorer"/>
                <meta name="viewport" content="width=device-width, initial-scale=1"/>
                <link rel="icon" href="/favicon.ico"/>
            </Head>
            <div className={styles.container}>
                <Navigation handleOpen={handleOpen}/>
                <Sidebar selectTable={selectTable}/>
                <div className={styles.pageContent}>
                    <Cards data={state.cardData}
                           handleOpen={handleOpen} selectTable={selectTable}/>
                    <DataTable handleOpen={handleOpen} newData={focus} index={index} selectTable={selectTable}/>
                    <DetailModal getModalData={getModalData} state={modalState} setModalState={setModalState}
                                 blockPage={modalPage} setBlockPage={(page)=>setModalPage(page)} handleClose={handleClose}/>
                </div>
            </div>
        </>
    }
}