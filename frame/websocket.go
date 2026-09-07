package frame

import (
	"context"
	"net/http"
	"time"

	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/gorilla/websocket"
)

type WebSocket struct {
	addr string
	upgrade *websocket.Upgrader
	closed bool
}

func NewWebSocket(addr string) (*WebSocket, error) {
	return &WebSocket{addr: addr, upgrade: &websocket.Upgrader{}}, nil
}

func (s *WebSocket) Open(ctx context.Context, name string, timeoutPerRead time.Duration, timeoutTotal time.Duration) (*Frame, error) {
	received := make(chan []byte)
	srv := &http.Server{
		Addr: s.addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := s.upgrade.Upgrade(w, r, nil)
			if err != nil {
				log.Ctx(ctx).Err(err)
				return
			}
			defer conn.Close()
			for {
				select {
					case <- ctx.Done():
						return
				default:
					_, message, err := conn.ReadMessage()
					if err != nil {
						log.Ctx(ctx).Err(err)
						break
					}
					received <- message
				}
			}
		}),
	}
	go srv.ListenAndServe()
	f := NewFrame(
		func (Nctx context.Context) ([]byte, error) {
		select{
		case <- Nctx.Done():
			return nil, Nctx.Err()
		case data, ok := <- received:
			if !ok {
				return nil, ErrClosed
			}
			return data, nil
		default:
			return nil, ErrEmpty
		}
		},
		func ()  {
			if s.closed {
				return
			}
			err := srv.Close()
			if err != nil {
				log.Ctx(ctx).Err(err)
			}
			close(received)
			s.closed = true
		},
		func () string {
			return name
		},
		timeoutPerRead,
		timeoutTotal,
	)
	return f, nil
}
