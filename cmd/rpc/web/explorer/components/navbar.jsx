import Container from "react-bootstrap/Container";
import Navbar from "react-bootstrap/Navbar";
import { Form } from "react-bootstrap";
import { convertIfNumber } from "@/components/util";

export default function Navigation({ openModal }) {
  let q = "",
    urls = { discord: "https://discord.gg/pNcSJj7Wdh", x: "https://x.com/CNPYNetwork" };
  return (
    <>
      <Navbar sticky="top" data-bs-theme="light" className="nav-bar">
        <Container>
          <Navbar.Brand className="nav-bar-brand">
            <img 
              src="/scanopy.png" 
              alt="Scanopy Logo" 
              className="nav-bar-logo"
             />
          </Navbar.Brand>
          <div className="nav-bar-center">
            <Form onSubmit={() => openModal(convertIfNumber(q), 0)}>
              <Form.Control
                type="search"
                className="main-input nav-bar-search me-2"
                placeholder="search by address, hash, or height"
                onChange={(e) => {
                  q = e.target.value;
                }}
              />
            </Form>
          </div>
          <a href={urls.discord}>
            <div id="nav-social-icon1" className="nav-social-icon justify-content-end" />
          </a>
          <a href={urls.x}>
            <div id="nav-social-icon2" className="nav-social-icon justify-content-end" />
          </a>
        </Container>
      </Navbar>
    </>
  );
}
