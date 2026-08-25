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
	return &Frame{
		started: false,
		startAt: time.Now(),
		endAt: time.Now(),
		bufferEmptyTimeout: time.Now(),
		timeoutPerRead: timeoutPerRead,
		timeoutTotal: timeoutTotal,
		read: func () ([]byte, error) {
		for {
			// blocking
			select{
			case <- ctx.Done():
				return nil, ctx.Err()
			default:
				msg, err := rb.ReadMsg()
				if err != nil {
					if errors.Is(err, ringbuffer.ErrBufferEmpty) {
						return nil, ErrEmpty
					}else if errors.Is(err, ringbuffer.ErrClosed) {
						return nil, ErrClosed
					} else {
						return nil, err
					}
				}
				return msg, nil
			}
		}
		},
		close: func ()  {
			err := rb.Close()
			if err != nil {
				log.Ctx(ctx).Err(err)
			}
		},
		name: func () string {
			return path.Join(i.path, name)
		},
	}, nil
}

func pathExists(path string) bool {
    _, err := os.Stat(path)
    if err == nil {
        return true
    }
    return !os.IsNotExist(err)
}