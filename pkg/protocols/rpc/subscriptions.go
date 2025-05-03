// pkg/protocols/rpc/subscription.go
package rpc

import (
	"bytes"
	"context"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/goccy/go-json"
	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
)

// Add a buffer pool
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Subscription represents a client's subscription.
type Subscription struct {
	ID         string
	Method     string
	Params     interface{}
	CreatedAt  time.Time
	Connection *ClientConnection
}

// ClientConnection represents a client's connection and subscriptions.
type ClientConnection struct {
	Conn          gnet.Conn
	Subscriptions map[string]*Subscription
	Mu            deadlock.Mutex // Protects Subscriptions
	Ctx           context.Context
	Cancel        context.CancelFunc
}

// BroadcastEvent broadcasts an event to all relevant subscribers.
func (s *Server) BroadcastEvent(eventType string, eventData interface{}) {
	var wg sync.WaitGroup
	workerPool := make(chan struct{}, 100) // Limit to 100 concurrent workers

	s.webSocketHandler.clientConnections.Range(func(key, value interface{}) bool {
		clientConn := value.(*ClientConnection)
		clientConn.Mu.Lock()
		subscriptions := make([]*Subscription, 0, len(clientConn.Subscriptions))
		for _, sub := range clientConn.Subscriptions {
			if sub.Method == eventType {
				subscriptions = append(subscriptions, sub)
			}
		}
		clientConn.Mu.Unlock()

		if len(subscriptions) > 0 {
			wg.Add(1)
			workerPool <- struct{}{} // Acquire a slot
			go func(conn gnet.Conn, subs []*Subscription) {
				defer wg.Done()
				defer func() { <-workerPool }() // Release the slot
				for _, sub := range subs {
					// Create a notification message
					notification := Notification{
						JSONRPC: "2.0",
						Method:  sub.Method,
						Params: NotificationParams{
							Subscription: sub.ID,
							Result:       eventData,
						},
					}
					// Send the notification
					s.sendNotification(conn, notification)
				}
			}(clientConn.Conn, subscriptions)
		}
		return true
	})
	wg.Wait()
}

// sendNotification sends a notification to a client over WebSocket.
func (s *Server) sendNotification(conn gnet.Conn, notification Notification) {
	// Get a buffer from the pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Create a JSON encoder
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(notification)
	if err != nil {
		s.logger.Error("Failed to encode notification", zap.Error(err))
		return
	}

	// Send the notification over WebSocket
	err = wsutil.WriteServerMessage(conn, ws.OpText, buf.Bytes())
	if err != nil {
		s.logger.Error("Failed to send notification", zap.Error(err))
	}
}
