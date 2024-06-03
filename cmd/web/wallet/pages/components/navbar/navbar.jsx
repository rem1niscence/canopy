import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar';
import {Dropdown, DropdownButton, Nav, NavDropdown, OverlayTrigger, Tooltip} from "react-bootstrap";

const navbarIconsAndTip = [
    {"icon":"./account.png", "tip":"accounts"},
    {"icon":"./gov.png", "tip":"governance"},
    {"icon":"./dashboard.png", "tip":"monitor"},
]

function Navigation({keystore, setActiveKey, activeKey, setContentIndex}) {
    return (<Navbar sticky="top" data-bs-theme="light"
                    style={{backgroundColor: "white", borderBottom:"1px solid lightgray", paddingBottom:"15px", position: "fixed", width: "100%"}}>
        <Container style={{paddingLeft: "20px"}}>
            <Navbar.Brand style={{fontWeight: "bold", color: "black"}} href="#home">my <span
                style={{color: "#32908F"}}>canopy </span>
                wallet</Navbar.Brand>
            <div id="nav-dropdown-container">
                <NavDropdown
                    align="end"
                    id="nav-dropdown"
                    title={<span>{Object.keys(keystore)[activeKey]}<img id="dropdown-image" src="./key.png"/></span>}
                >
                    {Object.keys(keystore).map(function (key, i) {
                        return <NavDropdown.Item onClick={()=>setActiveKey(i)} key={i} className="nav-dropdown-item" href="#">{key}</NavDropdown.Item>
                    })}
                </NavDropdown>
            </div>
            <div style={{margin: "0 auto", width: "25%"}}>
                <Nav style={{border:"0"}} justify variant="tabs" defaultActiveKey="0">
                    {navbarIconsAndTip.map(function(data, idx){
                        return (
                            <Nav.Item key={idx} onClick={()=> setContentIndex(idx)}>
                                <OverlayTrigger placement="right" delay={{show: 250, hide: 0}} overlay={<Tooltip id="button-tooltip">{data.tip}</Tooltip>}>
                                    <Nav.Link eventKey={idx.toString()} >
                                        <img className="navbar-image-link" src={data.icon} alt={data.tip}/>
                                    </Nav.Link>
                                </OverlayTrigger>
                            </Nav.Item>
                        )
                    })}
                </Nav>
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