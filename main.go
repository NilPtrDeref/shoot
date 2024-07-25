package main

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/nilptrderef/shoot/game"
	"github.com/nilptrderef/shoot/templates"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

var upgrade = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

func main() {
	g := game.NewGame()

	router := mux.NewRouter()

	router.HandleFunc("/", HandleIndex()).Methods("GET")
	router.HandleFunc("/room/list", ListRooms(g)).Methods("GET")
	router.HandleFunc("/room/{room}", LoadRoom()).Methods("GET")
	router.HandleFunc("/room/{room}/ws", JoinRoom(g)).Methods("GET")

	router.PathPrefix("/public").Handler(
		http.StripPrefix("/public", http.FileServer(http.Dir("./public/"))),
	).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port

	server := http.Server{
		Addr:    addr,
		Handler: router,
	}

	logrus.Infof("Server started on port %s", port)
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func HandleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := templates.Index().Render(r.Context(), w)
		if err != nil {
			logrus.WithError(err).Error("failed to render index page")
		}
	}
}

func ListRooms(g *game.Game) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := templates.Rooms(g.Rooms).Render(r.Context(), w)
		if err != nil {
			logrus.WithError(err).Error("failed to render room list")
		}
	}
}

func LoadRoom() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["room"]
		err := templates.Game(id).Render(r.Context(), w)
		if err != nil {
			logrus.WithError(err).Error("failed to render game room")
		}
	}
}

func JoinRoom(g *game.Game) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["room"]
		room := g.GetRoom(id)
		if room == nil {
			http.Error(w, `{"error": "room not found"}`, http.StatusNotFound)
			return
		}

		conn, err := upgrade.Upgrade(w, r, nil)
		if err != nil {
			logrus.WithError(err).Error("Failed to upgrade connection")
			return
		}

		err = room.AddPlayer(conn)
		if err != nil {
			http.Error(w, `{"error": "room is full"}`, http.StatusForbidden)
			return
		}
	}
}
