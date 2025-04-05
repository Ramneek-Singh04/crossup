package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

// --------------------
// JSON Message Protocol
// --------------------

// Message defines the structure for all messages.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// sendMessage marshals and sends a JSON message over the WebSocket.
func sendMessage(conn *websocket.Conn, messageType string, data interface{}) error {
	msg := Message{
		Type: messageType,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	msg.Data = dataBytes

	fullMessage, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, fullMessage)
}

// handleMessage processes an incoming JSON message and updates the game state.
func handleMessage(msg []byte, gameState *GameState, playerID string, opponentConn *websocket.Conn) {
	var message Message
	err := json.Unmarshal(msg, &message)
	if err != nil {
		log.Println("Error unmarshaling message:", err)
		return
	}

	switch message.Type {
	case "board_update":
		var data struct {
			Cell   string `json:"cell"`
			Letter string `json:"letter"`
		}
		if err := json.Unmarshal(message.Data, &data); err != nil {
			log.Println("Error parsing board_update data:", err)
			return
		}
		log.Printf("Board update from %s: cell %s set to %s\n", playerID, data.Cell, data.Letter)
		// Update the game state for the respective player.
		if gameState != nil {
			if playerID == "player1" {
				gameState.Player1State[data.Cell] = data.Letter
			} else if playerID == "player2" {
				gameState.Player2State[data.Cell] = data.Letter
			}
		}
		// Broadcast the update to the opponent.
		if opponentConn != nil {
			if err := sendMessage(opponentConn, "board_update", data); err != nil {
				log.Println("Error broadcasting board_update:", err)
			}
		}
	case "heartbeat":
		log.Printf("Heartbeat received from %s\n", playerID)
	case "game_over":
		var data struct {
			Winner string `json:"winner"`
		}
		if err := json.Unmarshal(message.Data, &data); err != nil {
			log.Println("Error parsing game_over data:", err)
			return
		}
		if gameState != nil {
			gameState.IsFinished = true
		}
		log.Printf("Game over received from %s, winner: %s\n", playerID, data.Winner)
		// Broadcast game_over to the opponent.
		if opponentConn != nil {
			if err := sendMessage(opponentConn, "game_over", data); err != nil {
				log.Println("Error broadcasting game_over:", err)
			}
		}
	default:
		log.Println("Received unknown message type:", message.Type)
	}
}

// --------------------
// Data Structures
// --------------------

// Note: The Puzzle struct defined here will be the same type as in your db.go.
type Puzzle struct {
	Grid    string            `json:"grid"`
	Clues   map[string]string `json:"clues"`
	Answers map[string]string `json:"answers"`
}

// GameState represents the current state of a game.
type GameState struct {
	Puzzle       Puzzle            `json:"puzzle"`
	Player1State map[string]string `json:"player1_state"`
	Player2State map[string]string `json:"player2_state"`
	StartTime    time.Time         `json:"start_time"`
	IsFinished   bool              `json:"is_finished"`
}

// Define a dummy puzzle with 7-Across and 7-Down in case DB loading fails.
var dummyPuzzle = Puzzle{
	Grid: "dummy grid layout", // Replace with an actual grid representation if needed.
	Clues: map[string]string{
		"7-Across": "Example clue for 7-Across",
		"7-Down":   "Example clue for 7-Down",
	},
	Answers: map[string]string{
		"7-Across": "EXAMPLE",
		"7-Down":   "EXAMPLE",
	},
}

// --------------------
// WebSocket & Matchmaking
// --------------------

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Player represents a connected player.
type Player struct {
	ID   string
	Conn *websocket.Conn
}

// Match represents a game match between two players.
type Match struct {
	Player1 *Player
	Player2 *Player
}

var waitingPlayer *Player
var waitingMutex sync.Mutex

// wsHandler upgrades the connection, handles matchmaking, and manages game state.
func wsHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Create a new player using RemoteAddr as a simple unique ID.
	player := &Player{
		ID:   c.Request.RemoteAddr,
		Conn: conn,
	}

	waitingMutex.Lock()
	if waitingPlayer == nil {
		waitingPlayer = player
		waitingMutex.Unlock()
		log.Println("Player waiting:", player.ID)
		// Keep connection alive while waiting.
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("Player disconnected while waiting:", player.ID)
				waitingMutex.Lock()
				if waitingPlayer != nil && waitingPlayer.ID == player.ID {
					waitingPlayer = nil
				}
				waitingMutex.Unlock()
				return
			}
			// Process messages (like heartbeat) while waiting.
			handleMessage(msg, nil, "waiting", nil)
		}
	} else {
		// A waiting player is available; create a match.
		opponent := waitingPlayer
		waitingPlayer = nil
		waitingMutex.Unlock()

		match := Match{
			Player1: opponent,
			Player2: player,
		}
		log.Println("Match created between", match.Player1.ID, "and", match.Player2.ID)

		// ------------------------------
		// Connect to DB and Load Puzzle
		// ------------------------------
		var loadedPuzzle Puzzle

		db, err := ConnectDB()
		if err != nil {
			log.Println("Error connecting to DB:", err)
			// Fallback to dummy puzzle if there's a connection error.
			loadedPuzzle = dummyPuzzle
		} else {
			defer db.Close()
			loadedPuzzle, err = LoadPuzzle(db, 1) // Load puzzle with ID 1
			if err != nil {
				log.Println("Error loading puzzle:", err)
				// Fallback to dummy puzzle if the load fails.
				loadedPuzzle = dummyPuzzle
			}
		}

		// ------------------------------
		// Initialize Game State with the Loaded Puzzle
		// ------------------------------
		gameState := &GameState{
			Puzzle:       loadedPuzzle,
			Player1State: make(map[string]string),
			Player2State: make(map[string]string),
			StartTime:    time.Now(),
			IsFinished:   false,
		}

		// Prepare the game_start message with puzzle data and start time.
		startData := struct {
			Puzzle    Puzzle    `json:"puzzle"`
			StartTime time.Time `json:"start_time"`
		}{
			Puzzle:    loadedPuzzle,
			StartTime: gameState.StartTime,
		}

		// Send the game_start message to both players.
		if err := sendMessage(match.Player1.Conn, "game_start", startData); err != nil {
			log.Println("Error sending game_start to Player1:", err)
		}
		if err := sendMessage(match.Player2.Conn, "game_start", startData); err != nil {
			log.Println("Error sending game_start to Player2:", err)
		}

		// Use goroutines to handle messages from both players concurrently.
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for {
				_, msg, err := match.Player1.Conn.ReadMessage()
				if err != nil {
					log.Println("Player1 disconnected:", err)
					return
				}
				// Handle message from Player1; broadcast updates to Player2.
				handleMessage(msg, gameState, "player1", match.Player2.Conn)
			}
		}()

		go func() {
			defer wg.Done()
			for {
				_, msg, err := match.Player2.Conn.ReadMessage()
				if err != nil {
					log.Println("Player2 disconnected:", err)
					return
				}
				// Handle message from Player2; broadcast updates to Player1.
				handleMessage(msg, gameState, "player2", match.Player1.Conn)
			}
		}()

		wg.Wait()
	}
}

func main() {
	router := gin.Default()
	// Health endpoint for basic connectivity testing.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// WebSocket endpoint.
	router.GET("/ws", wsHandler)
	log.Println("Server starting on port 8080")
	router.Run(":8080")
}
