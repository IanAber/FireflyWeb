package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

/**
Start the Web Socket server. This sends out data to all subscribers on a regular schedule so subscribers don't need to poll for updates.
*/
func startDataWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		signal.L.Lock()   // Get the signal and lock it.
		signal.Wait()     // Wait for it to be signalled again. It is unlocked while we wait then locked again before returning
		signal.L.Unlock() // Unlock it

		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			//			log.Println("Failed to get the values websocket writer - ", err)
			return
		}
		var sJSON = getMinJsonStatus()
		_, err = fmt.Fprint(w, sJSON)
		if err != nil {
			log.Println("failed to write the values message to the websocket - ", err)
			return
		}
		if err := w.Close(); err != nil {
			//			log.Println("Failed to close the values websocket writer - ", err)
			return
		}
	}
}
