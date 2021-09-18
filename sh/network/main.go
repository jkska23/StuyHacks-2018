package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
	"sync"
	"io/ioutil"
	"strings"
	"math/rand"
	"time"
	"encoding/json"
)
var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type STATE int
const (
	INPUTTING STATE = iota
	JUDGING
)
type CONN struct {
	Conn *websocket.Conn
	Username string
	Hand []string
	mu *sync.Mutex
}
func (c *CONN) Send(messageType int, message []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, message)
}

type GAME struct {
	State STATE
	Connections map[int]*CONN
	Judging int
	Inputs []struct{Username, Card string}
	Blackcard string
	Started bool
}
func NewGame() *GAME {
	game := GAME{
		State: INPUTTING,
		Connections: make(map[int]*CONN, 0),
		Judging: 0,
		Inputs: make([]struct{Username, Card string}, 0),
		Started: false,
	}
	return &game
}

var BLACK []string
var WHITE []string
var BLACKCARD string
type GAMES struct {
	GAMES map[string]*GAME
	Mu *sync.Mutex
}
func (g *GAMES) Read() map[string]*GAME {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	return g.GAMES
}
func (g *GAMES) Write(newGAMES map[string]*GAME) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.GAMES = newGAMES
}

var ALLGAMES = GAMES{make(map[string]*GAME, 0), &sync.Mutex{}}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	var currGame *GAME
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//TODO: proper error handling
		log.Println(err)
		return
	}
	games := make([]string, 0, len(ALLGAMES.Read()))
	for game, _ := range ALLGAMES.Read() {
		games = append(games, game)
	}
	encodedGames, _ := json.Marshal(games)
	conn.WriteMessage(websocket.TextMessage, encodedGames)
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		log.Println(err)
		return
	}
	if _, exists := ALLGAMES.Read()[string(message)]; !exists {
		ALLGAMES.Mu.Lock()
		ALLGAMES.GAMES[string(message)] = NewGame()
		ALLGAMES.Mu.Unlock()
	}
	currGame = ALLGAMES.Read()[string(message)]
	messageType, message, err = conn.ReadMessage()
	if err != nil {
		log.Println(err)
		return
	}
	for _, c := range currGame.Connections {
		if c.Username == string(message) {
			conn.WriteMessage(messageType, []byte("[\"Your username is already taken. Please try again with a different username\"]"))
			conn.WriteMessage(messageType, []byte("dup"))
			conn.WriteMessage(messageType, []byte{})
			return
		}
	}
	number := len(currGame.Connections)
	hand := make([]string, 0, 5)
	for len(hand) < cap(hand) {
		hand = append(hand, WHITE[0])
		WHITE = WHITE[1:]
	}
	currGame.Connections[number] = &CONN{conn, string(message), hand, &sync.Mutex{}}
	thisConn := currGame.Connections[number]
	encodedHand, _ := json.Marshal(hand)
	thisConn.Send(messageType, encodedHand)
	if len(currGame.Connections) == 1 && !currGame.Started {
		for len(currGame.Connections) == 1 {}
		currGame.Started = true
		currGame.State = INPUTTING
		BLACKCARD = BLACK[rand.Intn(len(BLACK))]
		for i, connection := range currGame.Connections {
			if i == currGame.Judging {
				connection.Send(messageType, []byte("judge"))
				connection.Send(messageType, []byte(BLACKCARD))
			}
		}
	} else if currGame.Started {
		for currGame.State == JUDGING {}
		thisConn.Send(messageType, []byte("play"))
		thisConn.Send(messageType, []byte(BLACKCARD))
		encodedHand, _ := json.Marshal(hand)
		thisConn.Send(messageType, encodedHand)
	}
	for {
		messageType, message, err = conn.ReadMessage()
		if _, k := err.(*websocket.CloseError); k {
			delete(currGame.Connections, number)
			if number == currGame.Judging {
				for _, connection := range currGame.Connections {
					connection.Send(messageType, []byte("Nobody"))
					connection.Send(messageType, []byte("Nothing, because the judge left"))
				}
				currGame.Judging += 1
				if currGame.Judging >= len(currGame.Connections) {
					currGame.Judging = 0
				}
				currGame.State = INPUTTING
				BLACKCARD = BLACK[rand.Intn(len(BLACK))]
				for i, connection := range currGame.Connections {
					if i == currGame.Judging {
						connection.Send(messageType, []byte("judge"))
						connection.Send(messageType, []byte(BLACKCARD))
					} else {
						connection.Send(messageType, []byte("play"))
						connection.Send(messageType, []byte(BLACKCARD))
						encodedHand, _ := json.Marshal(connection.Hand)
						connection.Send(messageType, encodedHand)
					}
				}
			}
			conn.Close()
			return
		}
		switch currGame.State {
		case INPUTTING:
			if currGame.Judging != number {
				if len(message) > 1 && message[len(message)-1] == 10 {
					message = message[0:len(message)-1]
				}
				choice, _ := strconv.Atoi(string(message))
				//TODO: error handle
				currGame.Connections[currGame.Judging].Send(messageType, []byte(hand[choice-1]))
				currGame.Inputs = append(currGame.Inputs, struct{Username, Card string}{currGame.Connections[number].Username, hand[choice-1]})
				WHITE = append(WHITE, hand[choice-1])
				hand = append(hand[:choice-1], hand[choice:]...)
				hand = append(hand, WHITE[0])
				WHITE = WHITE[1:]
				if len(currGame.Inputs) == len(currGame.Connections) - 1 {
					currGame.State = JUDGING
					currGame.Connections[currGame.Judging].Send(messageType, []byte("finished"))
				}
			}
		case JUDGING:
			if currGame.Judging == number {
				if len(message) > 1 && message[len(message)-1] == 10 {
					message = message[0:len(message)-1]
				}
				choice, _ := strconv.Atoi(string(message))
				//TODO: error handle
				for i, connection := range currGame.Connections {
					if i != number {
						connection.Send(messageType, []byte(currGame.Inputs[choice-1].Username))
						connection.Send(messageType, []byte(currGame.Inputs[choice-1].Card))
					}
				}
				currGame.Inputs = []struct{Username, Card string}{}
				currGame.Judging += 1
				if currGame.Judging >= len(currGame.Connections) {
					currGame.Judging = 0
				}
				currGame.State = INPUTTING
				BLACKCARD = BLACK[rand.Intn(len(BLACK))]
				for i, connection := range currGame.Connections {
					if i == currGame.Judging {
						connection.Send(messageType, []byte("judge"))
						connection.Send(messageType, []byte(BLACKCARD))
					} else {
						connection.Send(messageType, []byte("play"))
						connection.Send(messageType, []byte(BLACKCARD))
						encodedHand, _ := json.Marshal(connection.Hand)
						connection.Send(messageType, encodedHand)
					}
				}
			}
		}
	}
}
func main() {
	black, err := ioutil.ReadFile("black_cards.txt")
	white, err := ioutil.ReadFile("white_cards.txt")
	if err != nil {
		log.Fatal(err)
	}
	BLACK = strings.Split(string(black), "\n")
	WHITE = strings.Split(string(white), "\n")
	rand.Seed(time.Now().UTC().UnixNano())
	Shuffle(WHITE)
	http.HandleFunc("/", wsHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func Shuffle(slc []string) {
	N := len(slc)
	for i := 0; i < N; i++ {
		// choose index uniformly in [i, N-1]
		r := i + rand.Intn(N-i)
		slc[r], slc[i] = slc[i], slc[r]
	}
}
