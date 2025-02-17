import React, { useState, useEffect } from 'react';

const CanaLog = ({ text }) => {
  const [logs, setLogs] = useState([]);

  useEffect(() => {
    // 1. Decode HTML entities IMMEDIATELY when the prop changes
    const decodedText = text ? text.replace(/&#(\d+);/g, (match, code) => String.fromCharCode(code)) : "";
    const newLogs = parseLogs(decodedText); // Pass the DECODED text to parseLogs
    setLogs(newLogs);
  }, [text]);

  const parseLogs = (data) => {
    if (!data) return [];

    const lines = data.split('\n');
    const parsedLogs = lines.reduce((acc, line) => {
      let trimmedLine = line.trim();
      if (trimmedLine === "") return acc;

      // 1. Decode HTML entities (if present) - Do this EARLY
      trimmedLine = trimmedLine.replace(/&#(\d+);/g, (match, code) => String.fromCharCode(code));

      // 2. Remove ANSI escape codes FIRST - This is CRUCIAL
      const cleanedLine = trimmedLine.replace(/\x1b\[\d+m/g, '');


      // 3. Split by the *cleaned* timestamp (first 19 characters)
      const timestamp = cleanedLine.slice(0, 19).trim(); // Extract timestamp

      // 4. Extract msgtype and message
      const messageParts = cleanedLine.slice(19).trim().match(/^(DEBUG:|INFO:)\s(.*)$/);
      let msgtype = "";
      let message = cleanedLine.slice(19).trim(); // Default message if no match

      if (messageParts && messageParts.length === 3) {
        msgtype = messageParts[1];
        message = messageParts[2];
      }

      acc.push({ timestamp, msgtype, message });

      return acc;
    }, []);

    return parsedLogs;
  };

  return (
    <div className="canalog-container">
      {logs.map((log, index) => (
        <div key={index} className="canalog-row">
          <span className="canalog-label">{log.timestamp}</span>
          <span className={`canalog-msgtype ${log.msgtype === 'DEBUG:' ? 'canalog-debug' : log.msgtype === 'INFO:' ? 'canalog-info' : ''}`}>{log.msgtype}</span>
          <span className="canalog-message">{log.message}</span>
        </div>
      ))}
    </div>
  );
};

export default CanaLog;