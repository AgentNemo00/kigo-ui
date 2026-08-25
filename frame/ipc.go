package frame

import (
	"context"
	"errors"
	"os"
	"path"
	"time"

	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/EBWi11/mmap_ringbuffer"
)

type IPC struct {
	path string
	closed bool
}

func NewIPC(path string) (*IPC, error) {
	if !pathExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return nil, err
		}
	}
	return &IPC{path: path}, nil
}

func (i *IPC) Open(ctx context.Context, name string, bufferSize int, timeoutPerRead time.Duration, timeoutTotal time.Duration) (*Frame, error) {
	rb, err := ringbuffer.NewRingBuffer(path.Join(i.path, name), bufferSize+8, true)
    if err != nil {
        return nil, err
    }
	f := NewFrame(
		func (Nctx context.Context) ([]byte, error) {
		select{
		case <- Nctx.Done():
			return nil, Nctx.Err()
		default:
			msg, err := rb.ReadMsg()
			if err != nil {
				if errors.Is(err, ringbuffer.ErrBufferEmpty) {
					return nil, ErrEmpty
				}else if errors.Is(err, ringbuffer.ErrClosed) {
					return nil, ErrClosed
				}
				return nil, nil
			}
			return msg, nil
		}
		},
		func ()  {
			if i.closed {
				return
			}
			err := rb.Close()
			if err != nil {
				log.Ctx(ctx).Err(err)
			}
			i.closed = true
		},
		func () string {
			return path.Join(i.path, name)
		},
		timeoutPerRead,
		timeoutTotal,
	)
	return f, nil
}

func pathExists(path string) bool {
    _, err := os.Stat(path)
    if err == nil {
        return true
    }
    return !os.IsNotExist(err)
}