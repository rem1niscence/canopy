import Container from "react-bootstrap/Container";
import Navbar from "react-bootstrap/Navbar";
import { Nav, NavDropdown } from "react-bootstrap";
import { withTooltip } from "@/components/util";

const navbarIconsAndTip = [
  { icon: "./account.png", tip: "accounts" },
  { icon: "./gov.png", tip: "governance" },
  { icon: "./dashboard.png", tip: "monitor" },
];

const socials = [
  { url: "https://discord.gg/pNcSJj7Wdh", icon: "./discord-filled.png" },
  { url: "https://x.com/CNPYNetwork", icon: "./twitter.png" },
];

export default function Navigation({ keystore, setActiveKey, keyIdx, setNavIdx, height }) {
  return (
    <Navbar sticky="top" data-bs-theme="light" id="nav-bar">
      <Container id="nav-bar-container">
        <Navbar.Brand id="nav-bar-brand">
          <img 
                src="/wallet_logo.png" 
                alt="Wallet Logo" 
                className="nav-bar-logo"
              />
          </Navbar.Brand>
        <div id="nav-dropdown-container">
          <NavDropdown
            id="nav-dropdown"
            title={
              <>
                {Object.keys(keystore)[keyIdx]}
                <img alt="key" id="dropdown-image" src="./key.png" />
              </>
            }
          >
            {Object.keys(keystore).map((key, i) => (
              <NavDropdown.Item onClick={() => setActiveKey(i)} key={i} className="nav-dropdown-item">
                {key}
              </NavDropdown.Item>
            ))}
          </NavDropdown>
        </div>
        <div id="nav-bar-select-container">
          <Nav id="nav-bar-select" justify variant="tabs" defaultActiveKey="0">
            {navbarIconsAndTip.map((v, i) => (
              <Nav.Item key={i} onClick={() => setNavIdx(i)}>
                {withTooltip(
                  <Nav.Link eventKey={i.toString()}>
                    <img className="navbar-image-link" src={v.icon} alt={v.tip} />
                  </Nav.Link>,
                  v.tip,
                  i,
                  "bottom",
                )}
              </Nav.Item>
            ))}
          </Nav>
        </div>
        <a href={socials[0].url}>
          <div
            id="nav-social-icon-discord"
            style={{ backgroundImage: "url(" + socials[0].icon + ")" }}
            className="nav-social-icon"
          />
        </a>
        <a href={socials[1].url}>
          <div style={{ backgroundImage: "url(" + socials[1].icon + ")" }} className="nav-social-icon" />
        </a>
        <Navbar.Text className="ms-4">
          Latest height: <strong className="nav-bar-brand-highlight">{Math.max(height - 1, 1)}</strong>
        </Navbar.Text>
      </Container>
    </Navbar>
  );
}
