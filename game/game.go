package game

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	Width           = 768
	Height          = 768
	MoveDelta       = 5.0
	PlayerRadius    = 10.0
	BulletRadius    = 5.0
	RefreshRate     = 60 // Target FPS
	BulletMoveDelta = 400.0 / float64(RefreshRate)
	RespawnTime     = time.Second
	PingTime        = 10 * time.Second
	PongTimeout     = 12 * time.Second
	WriteDeadline   = 10 * time.Second
	Slots           = 4
)

func CheckCollision(player Player, bullet Bullet) bool {
	if player.ID == bullet.Owner {
		return false
	}

	// Calculate the distance between the centers of the circles
	dx := bullet.Position.X - player.Position.X
	dy := bullet.Position.Y - player.Position.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Check if the distance is less than the sum of the radii
	return distance < (PlayerRadius + BulletRadius)
}

type Game struct {
	Rooms []*Room
}

func NewGame() *Game {
	game := &Game{
		Rooms: []*Room{
			{ID: uuid.New().String(), Name: "Room 1", Slots: Slots, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan Message, 10)},
			{ID: uuid.New().String(), Name: "Room 2", Slots: Slots, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan Message, 10)},
			{ID: uuid.New().String(), Name: "Room 3", Slots: Slots, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan Message, 10)},
			{ID: uuid.New().String(), Name: "Room 4", Slots: Slots, players: []Player{}, RWMutex: &sync.RWMutex{}, receive: make(chan Message, 10)},
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

type Vector2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Movement struct {
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
}

type Bullet struct {
	Owner     string  `json:"owner"`
	Direction Vector2 `json:"direction"`
	Position  Vector2 `json:"position"`
}

func (b Bullet) OutOfBounds() bool {
	if b.Position.X-BulletRadius < 0 {
		logrus.WithField("bullet", b).Debug("bullet going off left side")
		return true
	}
	if b.Position.Y-BulletRadius < 0 {
		logrus.WithField("bullet", b).Debug("bullet going off top side")
		return true
	}
	if b.Position.X+BulletRadius > Width {
		logrus.WithField("bullet", b).Debug("bullet going off right side")
		return true
	}
	if b.Position.Y+BulletRadius > Height {
		logrus.WithField("bullet", b).Debug("bullet going off bottom side")
		return true
	}

	return false
}

type Event struct {
	Type      string `json:"type"`
	Sequence  int64  `json:"sequence"`
	*Movement `json:"movement"`
	*Bullet   `json:"bullet"`
}

type Message struct {
	ID    string
	Event Event
}

type Room struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slots       int    `json:"slots"`
	players     []Player
	bullets     []Bullet
	PlayerCount int `json:"player_count"`
	updates     bool
	receive     chan Message
	*sync.RWMutex
}

func (r *Room) PlayLoop() {
	last := time.Now()
	ticker := time.NewTicker(time.Second / RefreshRate)
	defer ticker.Stop()

	for {
		select {
		// TODO: Handle these messages
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
				r.Lock()
				player := r.FindPlayer(message.ID)
				if player != nil {
					player.Move(message.Event.Sequence, message.Event.Movement)
				}
				r.Unlock()

			case "fire":
				r.Lock()
				player := r.FindPlayer(message.ID)
				if player != nil && player.SpawnTime <= 0 && message.Event.Bullet != nil {
					bullet := *message.Event.Bullet
					bullet.Position.X = player.Position.X + bullet.Direction.X*BulletMoveDelta
					bullet.Position.Y = player.Position.Y + bullet.Direction.Y*BulletMoveDelta
					r.bullets = append(r.bullets, bullet)
				}
				r.Unlock()

			case "reskin":
				r.Lock()
				player := r.FindPlayer(message.ID)
				player.Hue = rand.Intn(360)
				r.updates = true
				r.Unlock()
			}

		case current := <-ticker.C:
			r.Lock()

			diff := current.Sub(last)
			last = current

			for i := range r.players {
				if r.players[i].SpawnTime > 0 {
					r.updates = true
					r.players[i].SpawnTime -= diff
					if r.players[i].SpawnTime < 0 {
						r.players[i].SpawnTime = 0
						r.players[i].Position.X = rand.Float64() * ((Width - (2 * PlayerRadius)) + PlayerRadius)
						r.players[i].Position.Y = rand.Float64() * ((Height - (2 * PlayerRadius)) + PlayerRadius)
					}
					continue
				}

				r.bullets = slices.DeleteFunc(r.bullets, func(b Bullet) bool {
					if CheckCollision(r.players[i], b) {
						r.players[i].SpawnTime = RespawnTime
						r.updates = true
						return true
					}
					return false
				})
			}

			for i := len(r.bullets) - 1; i >= 0; i-- {
				bullet := r.bullets[i]
				bullet.Position.X = bullet.Position.X + bullet.Direction.X*BulletMoveDelta
				bullet.Position.Y = bullet.Position.Y + bullet.Direction.Y*BulletMoveDelta
				r.bullets[i] = bullet
				r.updates = true
			}
			r.bullets = slices.DeleteFunc(r.bullets, func(b Bullet) bool {
				return b.OutOfBounds()
			})

			if !r.updates {
				r.Unlock()
				continue
			}
			logrus.Debug("updating clients")

			msg, err := json.Marshal(map[string]any{
				"type":    "update",
				"players": &r.players,
				"bullets": &r.bullets,
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
		ID:  uuid.New().String(),
		Hue: rand.Intn(360),
		Position: Vector2{
			X: rand.Float64() * ((Width - (2 * PlayerRadius)) + PlayerRadius),
			Y: rand.Float64() * ((Height - (2 * PlayerRadius)) + PlayerRadius),
		},
		radius: PlayerRadius,
		conn:   conn,
		room:   r,
		send:   make(chan []byte, 10),
		close:  make(chan struct{}),
	}
	r.players = append(r.players, player)
	r.PlayerCount = len(r.players)
	player.conn.WriteJSON(map[string]any{
		"type": "bootstrap",
		"id":   player.ID,
	})

	// Update all users to let them know a player connected
	msg, err := json.Marshal(map[string]any{
		"type":    "update",
		"players": &r.players,
		"bullets": &r.bullets,
	})
	if err == nil {
		for _, player := range r.players {
			player.send <- msg
		}
	} else {
		logrus.WithFields(
			logrus.Fields{
				"room": r.ID,
			},
		).WithError(err).Error("failed to marshal game state")
	}

	go player.Handle()
	return nil
}

func (r *Room) RemovePlayer(player *Player) {
	logrus.WithField("player", *player).Debug("removing player")
	r.Lock()
	defer r.Unlock()

	r.players = slices.DeleteFunc(r.players, func(p Player) bool {
		return p.ID == player.ID
	})
	r.PlayerCount = len(r.players)

	r.bullets = slices.DeleteFunc(r.bullets, func(b Bullet) bool {
		return b.Owner == player.ID
	})

	// Update all users to let them know a player disconnected
	msg, err := json.Marshal(map[string]any{
		"type":    "update",
		"players": &r.players,
		"bullets": &r.bullets,
	})
	if err == nil {
		for _, player := range r.players {
			player.send <- msg
		}
	} else {
		logrus.WithFields(
			logrus.Fields{
				"room": r.ID,
			},
		).WithError(err).Error("failed to marshal game state")
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

type Player struct {
	ID        string `json:"id"`
	conn      *websocket.Conn
	Hue       int           `json:"hue"`
	Sequence  int64         `json:"sequence"`
	Position  Vector2       `json:"position"`
	SpawnTime time.Duration `json:"spawn_time"`
	radius    float64
	room      *Room
	send      chan []byte
	close     chan struct{}
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
	p.Sequence = sequence

	if movement.Up {
		if p.Position.Y >= (p.radius + MoveDelta) {
			p.Position.Y -= MoveDelta
		} else {
			p.Position.Y = 0 + p.radius
		}
	}
	if movement.Down {
		if p.Position.Y <= Height-(p.radius+MoveDelta) {
			p.Position.Y += MoveDelta
		} else {
			p.Position.Y = Height - p.radius
		}
	}
	if movement.Left {
		if p.Position.X >= (p.radius + MoveDelta) {
			p.Position.X -= MoveDelta
		} else {
			p.Position.X = 0 + p.radius
		}
	}
	if movement.Right {
		if p.Position.X <= Width-(p.radius+MoveDelta) {
			p.Position.X += MoveDelta
		} else {
			p.Position.X = Width - p.radius
		}
	}

	p.room.updates = true
}
