import React, { useState } from "react";
import useWebSocket from "./useWebSocket";

function App() {
  // Replace the URL with your backend WebSocket endpoint.
  const { messages, sendMessage } = useWebSocket("ws://localhost:8080/ws");
  const [input, setInput] = useState("");

  const handleSend = () => {
    // You can send a string or a JSON string.
    sendMessage(input);
    setInput("");
  };

  return (
    <div>
      <h1>Competitive Crossword WebSocket</h1>
      <div>
        <input
          type="text"
          placeholder="Type a message..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <button onClick={handleSend}>Send Message</button>
      </div>
      <div>
        <h2>Received Messages:</h2>
        <ul>
          {messages.map((msg, index) => (
            <li key={index}>{msg}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default App;
