import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar';
import {Form} from "react-bootstrap";

let text = ""

function convertIfNumber(str) {
    if (!isNaN(str) && !isNaN(parseFloat(str))) {
        return Number(str)
    } else {
        return str
    }
}

function handleSubmit(event, openModal) {
    event.preventDefault();
    openModal(convertIfNumber(text), 0)
}

function handleChange(event) {
    text = event.target.value
}

function Navigation({handleOpen}) {
    return (<Navbar sticky="top" data-bs-theme="light"
                    style={{overflow: "hidden", backgroundColor: "#f1f5f9", position: "fixed", width: "100%"}}>
        <Container style={{paddingLeft: "20px"}}>
            <Navbar.Brand style={{fontWeight: "bold", color: "black"}} href="#home"><span
                style={{color: "#32908F"}}>scan</span>opy
                explorer</Navbar.Brand>
            <div style={{margin: "0 auto", width: "50%", textAlign: "center"}}>
                <Form onSubmit={(event) => handleSubmit(event, handleOpen)} className="d-flex">
                    <Form.Control style={{margin: "0 auto", width: "100%", textAlign: "center"}}
                                  type="search"
                                  placeholder="search by address, hash, or height"
                                  className="me-2"
                                  aria-label="Search"
                                  onChange={(event) => handleChange(event)}
                    />
                </Form>
            </div>
            <a href="https://discord.com">
                <div style={{
                    backgroundImage: "url(./discord-filled.png)",
                    backgroundSize: "cover",
                    marginRight: "10px",
                    marginTop: "3px",
                    width: "30px",
                    height: "30px"
                }} className="nav-social-icon justify-content-end"/>
            </a>
            <a href="https://x.com">
                <div style={{
                    backgroundImage: "url(./twitter.png)",
                    backgroundSize: "cover",
                    width: "25px",
                    height: "25px"
                }}
                     className="nav-social-icon justify-content-end"/>
            </a>
        </Container>
    </Navbar>);
}

export default Navigation;