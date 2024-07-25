package game

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// TODO: Take sequence numbers from the client and send them back to the client
// TODO: On player add, send them their id

const (
	Width         = 768
	Height        = 768
	MoveDelta     = 5
	RefreshRate   = time.Second / 1
	PingTime      = 10 * time.Second
	PongTimeout   = 12 * time.Second
	WriteDeadline = 10 * time.Second
	Slots         = 1
)

type Game struct {
	Rooms []*Room
}

func NewGame() *Game {
	game := &Game{
		Rooms: []*Room{
			{ID: uuid.New().String(), Name: "Room 1", Slots: Slots, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan Message, 10)},
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

type Movement struct {
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
}

type Event struct {
	Type      string `json:"type"`
	Sequence  int64  `json:"sequence"`
	*Movement `json:"movement"`
}

type Message struct {
	ID    string
	Event Event
}

type Room struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slots       int64  `json:"slots"`
	players     []Player
	PlayerCount int64 `json:"player_count"`
	updates     bool
	receive     chan Message
	*sync.RWMutex
}

func (r *Room) PlayLoop() {
	ticker := time.NewTicker(RefreshRate)
	defer ticker.Stop()

	for {
		select {
		// TODO: Handle these messages
		// Movement
		// Shoot
		// Respawn
		// Quit
		case message, ok := <-r.receive:
			if !ok {
				return
			}

			logrus.WithFields(
				logrus.Fields{
					"event":  message.Event,
					"player": message.ID,
				},
			).Debug("received message")

			switch message.Event.Type {
			case "movement":
				r.RLock()
				player := r.FindPlayer(message.ID)
				if player != nil {
					player.Move(message.Event.Sequence, message.Event.Movement)
				}
				r.RUnlock()
			}

		case <-ticker.C:
			r.Lock()
			if !r.updates {
				r.Unlock()
				continue
			}
			logrus.Debug("updating clients")

			msg, err := json.Marshal(map[string]any{
				"type":    "update",
				"players": &r.players,
			})
			if err != nil {
				r.Unlock()
				logrus.WithFields(
					logrus.Fields{
						"room": r.ID,
					},
				).WithError(err).Error("failed to marshal game state")
				break
			}

			for _, player := range r.players {
				player.send <- msg
			}
			r.updates = false
			r.Unlock()
		}
	}
}

func (r *Room) AddPlayer(conn *websocket.Conn) error {
	r.Lock()
	defer r.Unlock()

	if r.PlayerCount >= r.Slots {
		return errors.New("room is full")
	}

	player := Player{
		ID: uuid.New().String(),
		Info: PlayerInfo{
			X:      0,
			Y:      0,
			radius: 10,
		},
		conn:  conn,
		room:  r,
		send:  make(chan []byte, 10),
		close: make(chan struct{}),
	}
	r.players = append(r.players, player)
	r.PlayerCount = int64(len(r.players))
	player.conn.WriteJSON(map[string]any{
		"type":    "bootstrap",
		"id":      player.ID,
		"players": &r.players,
	})
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

func (r *Room) FindPlayer(id string) *Player {
	index := slices.IndexFunc(r.players, func(player Player) bool {
		return player.ID == id
	})
	if index < 0 {
		return nil
	}
	return &r.players[index]
}

type PlayerInfo struct {
	Sequence int64 `json:"sequence"`
	X        int64 `json:"x"`
	Y        int64 `json:"y"`
	radius   int64
}

type Player struct {
	ID    string `json:"id"`
	conn  *websocket.Conn
	Info  PlayerInfo `json:"info"`
	room  *Room
	send  chan []byte
	close chan struct{}
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

		var event Event
		err = json.Unmarshal(message, &event)
		if err != nil {
			logrus.WithFields(
				logrus.Fields{
					"id":   p.ID,
					"room": p.room.ID,
				},
			).WithError(err).Error("unexpected message format")
			break
		}

		p.room.receive <- Message{p.ID, event}
		logrus.WithFields(logrus.Fields{
			"id":   p.ID,
			"room": p.room.ID,
			"msg":  string(message),
		}).Debugf("received message")
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

func (p *Player) Move(sequence int64, movement *Movement) {
	p.Info.Sequence = sequence

	if movement.Up {
		if p.Info.Y >= (p.Info.radius + MoveDelta) {
			p.Info.Y -= MoveDelta
		} else {
			p.Info.Y = 0 + p.Info.radius
		}
	}
	if movement.Down {
		if p.Info.Y <= Height-(p.Info.radius+MoveDelta) {
			p.Info.Y += MoveDelta
		} else {
			p.Info.Y = Height - p.Info.radius
		}
	}
	if movement.Left {
		if p.Info.X >= (p.Info.radius + MoveDelta) {
			p.Info.X -= MoveDelta
		} else {
			p.Info.X = 0 + p.Info.radius
		}
	}
	if movement.Right {
		if p.Info.X <= Width-(p.Info.radius+MoveDelta) {
			p.Info.X += MoveDelta
		} else {
			p.Info.X = Width - p.Info.radius
		}
	}

	p.room.updates = true
}
