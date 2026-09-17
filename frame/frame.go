package frame

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTimeout = fmt.Errorf("timeout per read")
	ErrTime = fmt.Errorf("timeout total")
	ErrClosed = fmt.Errorf("closed")
	ErrEmpty = fmt.Errorf("empty")
)

type (
	Read = func(context.Context) ([]byte, error)
	Close = func()
	Name = func() string
)

type Frame struct {
	read Read
	close Close
	name Name
	
	started 			bool 			// read at least one package

	startAt 			time.Time		// time since when it is listening

	bufferEmptyTimeout 	time.Time		// time since when the buffer is empty

	timeoutPerRead 		time.Duration	// timeout between packages
	timeoutTotal 		time.Duration	// timeout total
}

func NewFrame(read Read, close Close, name Name, packageTimeout time.Duration, absoluteTimeout time.Duration) *Frame {
	return &Frame{
		read: read,
		close: close,
		name: name,
		startAt: time.Time{},
		bufferEmptyTimeout: time.Time{},
		timeoutTotal: absoluteTimeout,
		timeoutPerRead: packageTimeout,

	}
}

func (f *Frame) Read(ctx context.Context) ([]byte, error) {
	for {
		select {
			case <- ctx.Done():
				return nil, ctx.Err()
		default:		// timeout between packages
			if f.timeoutPerRead != 0 && f.bufferEmptyTimeout.Add(f.timeoutPerRead).Before(time.Now()) && f.started {
				return nil, ErrTimeout
			}
			// timeout total
			if f.timeoutTotal != 0 && f.startAt.Add(f.timeoutTotal).Before(time.Now()) && f.started {
				// error timeout 
				return nil, ErrTime
			}
			// none blocking
			data, err := f.read(ctx)
			if errors.Is(err, ErrEmpty) {
				continue
			}
			if errors.Is(err, ErrClosed) {
				return nil, ErrClosed
			}
			if !f.started {
				f.started = true
				if f.startAt.IsZero() {
					f.startAt = time.Now()
				}
			}
			f.bufferEmptyTimeout = time.Now()
			return data, err
		}
	}
}

func (f *Frame) Close() {
	f.close()
}

func (f* Frame) Name() string {
	return f.name()
}
