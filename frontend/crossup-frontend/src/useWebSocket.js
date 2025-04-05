import { useEffect, useState, useRef } from "react";

function useWebSocket(url) {
    const [messages, setMessages] = useState([]);
    const wsRef = useRef(null);

    useEffect(() => {
        // Create a new WebSocket instance.
        const ws = new WebSocket(url);
        wsRef.current = ws;

        // On open, you might want to log or update state.
        ws.onopen = () => {
            console.log("WebSocket connection opened.");
        };

        // On message, update your state to store received messages.
        ws.onmessage = (event) => {
            console.log("Received message:", event.data);
            setMessages((prevMessages) => [...prevMessages, event.data]);
        };

        ws.onerror = (error) => {
            console.error("WebSocket error:", error);
        };

        ws.onclose = () => {
            console.log("WebSocket connection closed.");
        };

        // Clean up function: close the connection when the component unmounts.
        return () => {
            if (ws) ws.close();
        };
    }, [url]);

    // Helper function to send a message.
    const sendMessage = (msg) => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(msg);
        } else {
            console.warn("WebSocket is not open. Unable to send message:", msg);
        }
    };

    return { messages, sendMessage };
}

export default useWebSocket;
