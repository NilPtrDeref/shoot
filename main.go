package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	game := NewGame()

	router := mux.NewRouter()

	router.HandleFunc("/room/list", ListRooms(game)).Methods("GET")
	router.HandleFunc("/room/{room}/ws", JoinRoom(game)).Methods("GET")

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/"))).Methods("GET")

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

func ListRooms(game *Game) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(w)
		err := encoder.Encode(game.Rooms)
		if err != nil {
			logrus.WithError(err).Error("Failed to encode rooms out to user")
		}
	}
}

func JoinRoom(game *Game) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["room"]
		room := game.GetRoom(id)
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
