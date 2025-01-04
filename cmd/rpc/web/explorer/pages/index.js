import Navigation from "@/components/navbar";
import Sidebar from "@/components/sidebar";
import DTable from "@/components/table"
import DetailModal from "@/components/modal"
import Cards from "@/components/cards";
import {useEffect, useState} from "react";
import {Spinner, Toast, ToastContainer} from "react-bootstrap";
import {getCardData, getTableData, getModalData, Orders} from "@/components/api";

export default function Home() {
    const [state, setState] = useState({
        loading: true,
        cardData: {},
        category: 0,
        tablePage: 0,
        tableData: {},
        showToast: false,

        sortColumn: null,
        sortDirection: 'asc',
        filterText: '',
        committee: 1,

        modalState: {
            show: false,
            page: 0,
            query: "",
            data: {},
            accOnly: false,
        }
    });

    function getCardAndTableData(setLoading) {
        Promise.all([getTableData(state.tablePage, state.category, state.committee), getCardData()]).then((values) => {
            if (setLoading) {
                return setState({...state,  loading: false, tableData: values[0], cardData: values[1]})
            }
            return setState({...state, tableData: values[0], cardData: values[1]})
        });
    }

    async function openModal(query) {
        setState({...state, modalState: {show: true, query: query, page: 0, accOnly: false, data: await getModalData(query, 0)}})
    }

    async function selectTable(category, page, committee) {
        if (committee == null) {
            committee = state.committee
        }
        setState({...state, committee: committee, category: category, tablePage: page, tableData: await getTableData(page, category, committee)})
    }

    useEffect(() => {
        const interval = setInterval(() => {
            getCardAndTableData(false)
        }, 4000);
        return () => clearInterval(interval);
    });
    if (state.loading || !state.cardData.blocks || Object.keys(state.cardData.blocks).length === 0 || state.cardData.blocks.results.length === 0) {
        getCardAndTableData(true)
        return <>
            <Spinner id="spinner"/>
            <br /> <br />
            <center><h3>Waiting to explore!</h3></center>
            <center><h7>... loading or no blocks yet ...</h7></center>
        </>
    } else {
        const props = {state, setState, openModal, selectTable}
        const onToastClose = () => setState({...state, showToast: false})
        return <>
            <div id="container">
                <Navigation {...props}/>
                <Sidebar {...props}/>
                <div id="pageContent">
                    <Cards {...props}/>
                    <DTable {...props} />
                    <DetailModal {...props} />
                    <ToastContainer id="toast" position={"bottom-end"}>
                        <Toast bg={"dark"} onClose={onToastClose} show={state.showToast} delay={2000} autohide>
                            <Toast.Body>Copied!</Toast.Body>
                        </Toast>
                    </ToastContainer>
                </div>
            </div>
        </>
    }
}