package frame

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/EBWi11/mmap_ringbuffer"
	"github.com/gogpu/compose"
)

type Socket struct {
	path string
	closed bool
}

func NewSocket(path string) (*Socket, error) {
	if !pathExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return nil, err
		}
	}
	return &Socket{path: path}, nil
}

func (s *Socket) Open(ctx context.Context, name string, bufferSize int, timeoutPerRead time.Duration, timeoutTotal time.Duration) (*Frame, error) {
	srv, err := compose.Listen(path.Join(s.path, fmt.Sprintf("%s.sock")))
	if err != nil {
		return nil, err
	}
	received := make(chan []byte)
	srv.OnFrame(func(f compose.Frame) {
		received <- f.Pixels
	})
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
			s.closed = true
		},
		func () string {
			return path.Join(s.path, name)
		},
		timeoutPerRead,
		timeoutTotal,
	)
	return f, nil
}
