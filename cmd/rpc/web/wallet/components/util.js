import { useState } from "react";
import { OverlayTrigger, Toast, ToastContainer, Tooltip, Form } from "react-bootstrap";

// getFormInputs() returns the form input based on the type
// account and validator is passed to assist with auto fill
export function getFormInputs(type, keyGroup, account, validator) {
  let amount = null;
  let netAddr = validator && validator.address ? validator.net_address : "";
  let delegate = validator && validator.address ? validator.delegate : false;
  let compound = validator && validator.address ? validator.compound : false;
  let output = validator && validator.address ? validator.output : "";
  let address = account != null ? account.address : "";
  let pubKey = keyGroup != null ? keyGroup.publicKey : "";
  let signer = account != null ? account.address : "";
  let committeeList = validator && validator.address ? validator.committees.join(",") : "";
  address = type !== "send" && validator && validator.address ? validator.address : address;
  address = type === "stake" && validator && validator.address ? "WARNING: validator already staked" : address;
  if (type === "edit-stake" || type === "stake") {
    amount = validator && validator.address ? validator.staked_amount : null;
  }
  let a = {
    privateKey: {
      placeholder: "opt: private key hex to import",
      defaultValue: "",
      tooltip: "the raw private key to import if blank - will generate a new key",
      label: "private_key",
      inputText: "key",
      feedback: "please choose a private key to import",
      required: false,
      type: "password",
      minLength: 64,
      maxLength: 128,
    },
    address: {
      placeholder: "the unique id of the account",
      defaultValue: address,
      tooltip: "the short public key id of the account",
      label: "sender",
      inputText: "address",
      feedback: "please choose an address to send the transaction from",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    pubKey: {
      placeholder: "public key of the node",
      defaultValue: pubKey,
      tooltip: "the public key of the validator",
      label: "pubKey",
      inputText: "pubKey",
      feedback: "please choose a pubKey to send the transaction from",
      required: true,
      type: "text",
      minLength: 96,
      maxLength: 96,
    },
    committees: {
      placeholder: "1, 22, 50",
      defaultValue: committeeList,
      tooltip: "comma separated list of committee chain IDs to stake for",
      label: "committees",
      inputText: "committees",
      feedback: "please input atleast 1 committee",
      required: true,
      type: "text",
      minLength: 1,
      maxLength: 200,
    },
    netAddr: {
      placeholder: "url of the node",
      defaultValue: netAddr,
      tooltip: "the url of the validator for consensus and polling",
      label: "net_address",
      inputText: "net-addr",
      feedback: "please choose a net address for the validator",
      required: true,
      type: "text",
      minLength: 5,
      maxLength: 50,
    },
    earlyWithdrawal: {
      placeholder: "early withdrawal rewards for 20% penalty",
      defaultValue: !compound,
      tooltip: "validator NOT reinvesting their rewards to their stake, incurring a 20% penalty",
      label: "earlyWithdrawal",
      inputText: "withdrawal",
      feedback: "please choose if your validator to earlyWithdrawal or not",
      required: true,
      type: "text",
      minLength: 4,
      maxLength: 5,
    },
    delegate: {
      placeholder: "validator delegation status",
      defaultValue: delegate,
      tooltip:
        "validator is passively delegating rather than actively validating. NOTE: THIS FIELD IS FIXED AND CANNOT BE UPDATED WITH EDIT-STAKE",
      label: "delegate",
      inputText: "delegate",
      feedback: "please choose if your validator is delegating or not",
      required: true,
      type: "text",
      minLength: 4,
      maxLength: 5,
    },
    rec: {
      placeholder: "recipient of the tx",
      defaultValue: "",
      tooltip: "the recipient of the transaction",
      label: "recipient",
      inputText: "recipient",
      feedback: "please choose a recipient for the transaction",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    amount: {
      placeholder: "amount value for the tx",
      defaultValue: amount,
      tooltip: "the amount of currency being sent / sold",
      label: "amount",
      inputText: "amount",
      feedback: "please choose an amount for the tx",
      required: true,
      type: "number",
      minLength: 1,
      maxLength: 100,
    },
    receiveAmount: {
      placeholder: "amount of counter asset to receive",
      defaultValue: amount,
      tooltip: "the amount of counter asset being received",
      label: "receiveAmount",
      inputText: "rec-amount",
      feedback: "please choose a receive amount for the tx",
      required: true,
      type: "number",
      minLength: 1,
      maxLength: 100,
    },
    orderId: {
      placeholder: "the id of the existing order",
      tooltip: "the unique identifier of the order",
      label: "orderId",
      inputText: "order-id",
      feedback: "please input an order id",
      required: true,
      type: "number",
      minLength: 1,
      maxLength: 100,
    },
    committeeId: {
      placeholder: "the id of the committee / counter asset",
      tooltip: "the unique identifier of the committee / counter asset",
      label: "committeeId",
      inputText: "commit-Id",
      feedback: "please input a committeeId id",
      required: true,
      type: "number",
      minLength: 1,
      maxLength: 100,
    },
    receiveAddress: {
      placeholder: "the address where the counter asset will be sent",
      tooltip: "the sender of the transaction",
      label: "receiveAddress",
      inputText: "rec-addr",
      feedback: "please choose an address to receive the counter asset to",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    buyersReceiveAddress: {
      placeholder: "the canopy address where CNPY will be received",
      tooltip: "the sender of the transaction",
      label: "receiveAddress",
      inputText: "rec-addr",
      feedback: "please choose an address to receive the CNPY",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    output: {
      placeholder: "output of the node",
      defaultValue: output,
      tooltip: "the non-custodial address where rewards and stake is directed to",
      label: "output",
      inputText: "output",
      feedback: "please choose an output address for the validator",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    signer: {
      placeholder: "signer of the transaction",
      defaultValue: signer,
      tooltip: "the signing address that authorizes the transaction",
      label: "signer",
      inputText: "signer",
      feedback: "please choose a signer address",
      required: true,
      type: "text",
      minLength: 40,
      maxLength: 40,
    },
    paramSpace: {
      placeholder: "",
      defaultValue: "",
      tooltip: "the category 'space' of the parameter",
      label: "param_space",
      inputText: "param space",
      feedback: "please choose a space for the parameter change",
      required: true,
      type: "select",
      minLength: 1,
      maxLength: 100,
    },
    paramKey: {
      placeholder: "",
      defaultValue: "",
      tooltip: "the identifier of the parameter",
      label: "param_key",
      inputText: "param key",
      feedback: "please choose a key for the parameter change",
      required: true,
      type: "select",
      minLength: 1,
      maxLength: 100,
    },
    paramValue: {
      placeholder: "",
      defaultValue: "",
      tooltip: "the newly proposed value of the parameter",
      label: "param_value",
      inputText: "param val",
      feedback: "please choose a value for the parameter change",
      required: true,
      type: "text",
      minLength: 1,
      maxLength: 100,
    },
    startBlock: {
      placeholder: "1",
      defaultValue: "",
      tooltip: "the block when voting starts",
      label: "start_block",
      inputText: "start blk",
      feedback: "please choose a height for start block",
      required: true,
      type: "number",
      minLength: 0,
      maxLength: 40,
    },
    endBlock: {
      placeholder: "100",
      defaultValue: "",
      tooltip: "the block when voting is counted",
      label: "end_block",
      inputText: "end blk",
      feedback: "please choose a height for end block",
      required: true,
      type: "number",
      minLength: 0,
      maxLength: 40,
    },
    memo: {
      placeholder: "opt: note attached with the transaction",
      defaultValue: "",
      tooltip: "an optional note attached to the transaction - blank is recommended",
      label: "memo",
      inputText: "memo",
      required: false,
      minLength: 0,
      maxLength: 200,
    },
    fee: {
      placeholder: "opt: transaction fee",
      defaultValue: "",
      tooltip: " a small amount of CNPY deducted from the account to process any transaction blank = default fee",
      label: "fee",
      inputText: "txn-fee",
      feedback: "please choose a valid number",
      required: false,
      type: "number",
      minLength: 0,
      maxLength: 40,
    },
    password: {
      placeholder: "key password",
      defaultValue: "",
      tooltip: "the password for the private key sending the transaction",
      label: "password",
      inputText: "password",
      feedback: "please choose a valid password",
      required: true,
      type: "password",
      minLength: 0,
      maxLength: 40,
    },
  };
  switch (type) {
    case "send":
      return [a.address, a.rec, a.amount, a.memo, a.fee, a.password];
    case "stake":
      return [
        a.address,
        a.pubKey,
        a.committees,
        a.netAddr,
        a.amount,
        a.delegate,
        a.earlyWithdrawal,
        a.output,
        a.signer,
        a.memo,
        a.fee,
        a.password,
      ];
    case "create_order":
      return [a.address, a.committeeId, a.amount, a.receiveAmount, a.receiveAddress, a.memo, a.fee, a.password];
    case "buy_order":
      return [a.address, a.buyersReceiveAddress, a.orderId, a.fee, a.password];
    case "edit_order":
      return [
        a.address,
        a.committeeId,
        a.orderId,
        a.amount,
        a.receiveAmount,
        a.receiveAddress,
        a.memo,
        a.fee,
        a.password,
      ];
    case "delete_order":
      return [a.address, a.committeeId, a.orderId, a.memo, a.fee, a.password];
    case "edit-stake":
      return [
        a.address,
        a.committees,
        a.netAddr,
        a.amount,
        a.earlyWithdrawal,
        a.output,
        a.signer,
        a.memo,
        a.fee,
        a.password,
      ];
    case "change-param":
      return [a.address, a.paramSpace, a.paramKey, a.paramValue, a.startBlock, a.endBlock, a.memo, a.fee, a.password];
    case "dao-transfer":
      return [a.address, a.amount, a.startBlock, a.endBlock, a.memo, a.fee, a.password];
    case "pause":
    case "unpause":
    case "unstake":
      return [a.address, a.signer, a.memo, a.fee, a.password];
    case "pass-and-addr":
      return [a.address, a.password];
    case "pass-and-pk":
      return [a.privateKey, a.password];
    case "pass-only":
      return [a.password];
    default:
      return [a.address, a.memo, a.fee, a.password];
  }
}

// placeholders is a dummy object to assist in the user experience and provide consistency
export const placeholders = {
  poll: {
    "PLACEHOLDER EXAMPLE": {
      proposalHash: "PLACEHOLDER EXAMPLE",
      proposalURL: "https://discord.com/channels/1310733928436600912/1323330593701761204",
      accounts: {
        approvedPercent: 38,
        rejectPercent: 62,
        votedPercent: 35,
      },
      validators: {
        approvedPercent: 76,
        rejectPercent: 24,
        votedPercent: 77,
      },
    },
  },
  pollJSON: {
    proposal: "canopy network is the best",
    endBlock: 100,
    URL: "https://discord.com/link-to-thread",
  },
  proposals: {
    "2cbb73b8abdacf233f4c9b081991f1692145624a95004f496a95d3cce4d492a4": {
      proposal: {
        parameter_space: "cons|fee|val|gov",
        parameter_key: "protocol_version",
        parameter_value: "example",
        start_height: 1,
        end_height: 1000000,
        signer: "4646464646464646464646464646464646464646464646464646464646464646",
      },
      approve: false,
    },
  },
  params: {
    parameter_space: "consensus",
    parameter_key: "protocol_version",
    parameter_value: "1/150",
    start_height: 1,
    end_height: 100,
    signer: "303739303732333263...",
  },
  rawTx: {
    type: "change_parameter",
    msg: {
      parameter_space: "cons",
      parameter_key: "block_size",
      parameter_value: 1000,
      start_height: 1,
      end_height: 100,
      signer: "1fe1e32edc41d688...",
    },
    signature: {
      public_key: "a88b9c0c7b77e7f8ac...",
      signature: "8f6d016d04e350...",
    },
    memo: "",
    fee: 10000,
  },
};

// numberWithCommas() formats a number with commas as thousand separators
export function numberWithCommas(x) {
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

// formatNumber() formats a number with optional division and compact notation
export function formatNumber(nString, div = true, cutoff = 1000000000000000) {
  if (nString == null) {
    return "zero";
  }
  if (div) {
    nString /= 1000000;
  }
  if (Number(nString) < cutoff) {
    return numberWithCommas(nString);
  }
  return Intl.NumberFormat("en", { notation: "compact", maximumSignificantDigits: 8 }).format(nString);
}

// copy() copies text to clipboard and triggers a toast notification
export function copy(state, setState, detail, toastText = "Copied!") {
  navigator.clipboard.writeText(detail);
  setState({ ...state, toast: toastText });
}

// renderToast() displays a toast notification with a customizable message
export function renderToast(state, setState) {
  return (
    <ToastContainer id="toast" position={"bottom-end"}>
      <Toast
        bg={"dark"}
        onClose={() => setState({ ...state, toast: "" })}
        show={state.toast != ""}
        delay={2000}
        autohide
      >
        <Toast.Body>{state.toast}</Toast.Body>
      </Toast>
    </ToastContainer>
  );
}

// onFormSubmit() handles form submission and passes form data to a callback
export function onFormSubmit(state, e, callback) {
  e.preventDefault();
  let r = {};
  for (let i = 0; ; i++) {
    if (!e.target[i] || !e.target[i].ariaLabel) {
      break;
    }
    r[e.target[i].ariaLabel] = e.target[i].value;
  }
  callback(r);
}

// withTooltip() wraps an element with a tooltip component
export function withTooltip(obj, text, key, dir = "right") {
  return (
    <OverlayTrigger
      key={key}
      placement={dir}
      delay={{ show: 250, hide: 400 }}
      overlay={<Tooltip id="button-tooltip">{text}</Tooltip>}
    >
      {obj}
    </OverlayTrigger>
  );
}

// getRatio() calculates the simplest ratio between two numbers
export function getRatio(a, b) {
  const [bg, sm] = a > b ? [a, b] : [b, a];
  for (let i = 1; i < 1000000; i++) {
    const d = sm / i;
    const res = bg / d;
    if (Math.abs(res - Math.round(res)) < 0.1) {
      return a > b ? `${Math.round(res)}:${i}` : `${i}:${Math.round(res)}`;
    }
  }
}

// objEmpty() checks if an object is null, undefined, or empty
export function objEmpty(o) {
  return !o || Object.keys(o).length === 0;
}

// disallowedCharacters is a string of characters that are not allowed in form inputs.
export const disallowedCharacters = ["\t", '"'];

// sanitizeInput removes disallowed characters from the given event target value.
// It is meant to be used as an onChange event handler.
export const sanitizeInput = (event) => {
  let value = event.target.value;

  disallowedCharacters.forEach((char) => {
    value = value.split(char).join("");
  });

  event.target.value = value;
};

// formatNumberInput is a function that formats a number input with commas as thousand separators.
// It is meant to be used as an onChange event handler.
export const formatNumberInput = (e) => {
  // Removes all non-digit characters and leading zeros from the input value.
  let input = e.target.value.replace(/[^\d]/g, "").replace(/^0/, "");
  // Check if the input is a number and is greater than 0, a regex is used as isNaN
  // may allow for unexpected input like empty strings or null values.
  if (/^\d+$/.test(input)) {
    input = numberWithCommas(input);
  }
  e.target.value = input;
};

// numberFromCommas is a function that converts a string of numbers formatted with commas
// as separators to a number.
export const numberFromCommas = (str) => {
  return Number(parseInt(str.replace(/,/g, ""), 10));
};
