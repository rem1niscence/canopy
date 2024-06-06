import Navigation from "@/pages/components/navbar/navbar";
import Sidebar from "@/pages/components/sidebar/sidebar";
import DataTable from "@/pages/components/table/table"
import DetailModal from "@/pages/components/modal/modal"
import Cards from "@/pages/components/cards/cards";
import {useEffect, useState} from "react";
import {Spinner, Toast, ToastContainer} from "react-bootstrap";
import {getCardData, getDataForTable, getModalData} from "@/pages/components/api";

export default function Home() {
    const [state, setState] = useState({
        loading: true,
        cardData: {},
        category: 0,
        tablePage: 0,
        tableData: {},
        showToast: false,
        modalState: {
            show: false,
            page: 0,
            query: "",
            data: {},
            accOnly: false,
        }
    });

    function getCardAndTableData(setLoading) {
        Promise.all([getDataForTable(state.tablePage, state.category), getCardData()]).then((values) => {
            if (setLoading) {
                return setState({...state,  loading: false, tableData: values[0], cardData: values[1]})
            }
            return setState({...state, tableData: values[0], cardData: values[1]})
        });
    }

    async function openModal(query) {
        setState({...state, modalState: {show: true, query: query, page: 0, accOnly: false, data: await getModalData(query, 0)}})
    }

    async function selectTable(category, page) {
        setState({...state, category: category, tablePage: page, tableData: await getDataForTable(page, category)})
    }

    useEffect(() => {
        const interval = setInterval(() => {
            getCardAndTableData(false)
        }, 4000);
        return () => clearInterval(interval);
    });

    if (state.loading) {
        getCardAndTableData(true)
        return <Spinner id="spinner"/>
    } else {
        const props = {state, setState, openModal, selectTable}
        const onToastClose = () => setState({...state, showToast: false})
        return <>
            <div id="container">
                <Navigation {...props}/>
                <Sidebar {...props}/>
                <div id="pageContent">
                    <Cards {...props}/>
                    <DataTable {...props} />
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