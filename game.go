package main

import (
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

// TODO: Add message passing to the websocket connection (Broadcast movements to the players in the room on a regular interval)
// TODO: Take sequence numbers from the client and send them back to the client
// TODO: On player add, send them their id

const (
	RefreshRate   = time.Second / 1
	PingTime      = 10 * time.Second
	PongTimeout   = 12 * time.Second
	WriteDeadline = 10 * time.Second
)

type Game struct {
	Rooms []*Room
}

func NewGame() *Game {
	game := &Game{
		Rooms: []*Room{
			{ID: uuid.New().String(), Name: "room 1", Slots: 4, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan []byte, 10)},
		},
	}

	for _, room := range game.Rooms {
		go room.PlayLoop()
	}

	return game
}

func (g *Game) GetRoom(id string) *Room {
	for _, room := range g.Rooms {
		if room.ID == id {
			return room
		}
	}
	return nil
}

type Room struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slots       int64  `json:"slots"`
	players     []Player
	PlayerCount int64 `json:"player_count"`
	receive     chan []byte
	*sync.RWMutex
}

func (r *Room) PlayLoop() {
	ticker := time.NewTicker(RefreshRate)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-r.receive:
			if !ok {
				return
			}

			logrus.WithField("msg", string(message)).Info("received message")
		case <-ticker.C:
			logrus.Info("updating clients")
		}
	}
}

func (r *Room) AddPlayer(conn *websocket.Conn) error {
	r.Lock()
	defer r.Unlock()

	if r.PlayerCount >= r.Slots {
		return errors.New("room is full")
	}

	player := Player{ID: uuid.New().String(), conn: conn, room: r, send: make(chan []byte, 10), close: make(chan struct{})}
	r.players = append(r.players, player)
	r.PlayerCount = int64(len(r.players))
	go player.Handle()
	return nil
}

func (r *Room) RemovePlayer(player *Player) {
	r.Lock()
	defer r.Unlock()

	for i, p := range r.players {
		if p.ID == player.ID {
			r.players = append(r.players[:i], r.players[i+1:]...)
			r.PlayerCount = int64(len(r.players))
			close(player.close)
			return
		}
	}
}

type Player struct {
	ID       string `json:"id"`
	conn     *websocket.Conn
	Sequence int64 `json:"sequence"`
	room     *Room
	send     chan []byte
	close    chan struct{}
}

func (p *Player) Handle() {
	logrus.WithFields(logrus.Fields{
		"id":   p.ID,
		"room": p.room.ID,
	}).Info("Player connected")
	go p.HandleReads()
	go p.HandleWrites()
}

func (p *Player) HandleReads() {
	defer p.conn.Close()
	defer p.room.RemovePlayer(p)

	err := p.conn.SetReadDeadline(time.Now().Add(PongTimeout))
	if err != nil {
		return
	}

	p.conn.SetReadLimit(1024)
	p.conn.SetPongHandler(func(string) error {
		err := p.conn.SetReadDeadline(time.Now().Add(PongTimeout))
		if err != nil {
			return err
		}
		return nil
	})

	for {
		_, message, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				logrus.WithFields(
					logrus.Fields{
						"id":   p.ID,
						"room": p.room.ID,
					},
				).WithError(err).Error("unexpected close error")
			}
			break
		}

		p.room.receive <- message
		logrus.WithFields(logrus.Fields{
			"id":   p.ID,
			"room": p.room.ID,
			"msg":  string(message),
		}).Infof("received message")
	}
}

func (p *Player) HandleWrites() {
	ticker := time.NewTicker(PingTime)
	defer ticker.Stop()
	defer p.conn.Close()

	for {
		select {
		case message, ok := <-p.send:
			if !ok {
				return
			}

			err := p.conn.SetWriteDeadline(time.Now().Add(WriteDeadline))
			if err != nil {
				return
			}

			err = p.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			err := p.conn.SetWriteDeadline(time.Now().Add(WriteDeadline))
			if err != nil {
				return
			}

			err = p.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		case <-p.close:
			return
		}
	}
}
