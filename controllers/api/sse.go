package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
)

// sseHub manages Server-Sent Events connections for real-time campaign
// event streaming (7.4). Each connected client receives a keep-alive
// comment every 15 seconds and JSON event payloads as they occur.

type sseClient struct {
	uid    int64
	events chan []byte
}

var (
	sseMu      sync.Mutex
	sseClients = map[*sseClient]struct{}{}
)

// SSEBroadcast sends an event to all connected SSE clients whose user_id
// matches the event's campaign owner. Called from model hooks.
func SSEBroadcast(uid int64, event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	sseMu.Lock()
	defer sseMu.Unlock()
	for c := range sseClients {
		if c.uid == uid {
			select {
			case c.events <- data:
			default:
				// Client is slow; drop event to avoid blocking.
			}
		}
	}
}

// EventStream handles GET /api/events/stream — an SSE endpoint that streams
// campaign events to the authenticated API user in real time (7.4).
func (as *Server) EventStream(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	_ = ctx.Get(r, "user").(models.User)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	client := &sseClient{
		uid:    uid,
		events: make(chan []byte, 64),
	}

	sseMu.Lock()
	sseClients[client] = struct{}{}
	sseMu.Unlock()

	defer func() {
		sseMu.Lock()
		delete(sseClients, client)
		sseMu.Unlock()
		close(client.events)
	}()

	// Keep-alive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case data := <-client.events:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
