package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Puzzle struct corresponds to your database table structure.
type Puzzle struct {
	ID      int               `json:"id"`
	Title   string            `json:"title"`
	Grid    string            `json:"grid"`
	Clues   map[string]string `json:"clues"`
	Answers map[string]string `json:"answers"`
	Created time.Time         `json:"created_at"`
}

// ConnectDB opens a connection to your PostgreSQL database.
func ConnectDB() (*sql.DB, error) {
	// Update the connection string with your credentials.
	connStr := "user=crossword_user password=your_password dbname=crossword_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	// Test the connection.
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// LoadPuzzle loads a puzzle from the database by ID.
func LoadPuzzle(db *sql.DB, puzzleID int) (Puzzle, error) {
	var puzzle Puzzle
	var cluesJSON, answersJSON []byte

	query := `SELECT id, title, grid, clues, answers, created_at FROM puzzles WHERE id = $1`
	row := db.QueryRow(query, puzzleID)
	if err := row.Scan(&puzzle.ID, &puzzle.Title, &puzzle.Grid, &cluesJSON, &answersJSON, &puzzle.Created); err != nil {
		return puzzle, err
	}

	// Unmarshal JSON fields.
	if err := json.Unmarshal(cluesJSON, &puzzle.Clues); err != nil {
		return puzzle, fmt.Errorf("error parsing clues: %v", err)
	}
	if err := json.Unmarshal(answersJSON, &puzzle.Answers); err != nil {
		return puzzle, fmt.Errorf("error parsing answers: %v", err)
	}
	return puzzle, nil
}
