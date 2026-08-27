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
	endAtAbsolute 		time.Time		// end at absolut
	bufferEmptyTimeout 	time.Time		
	timeoutPerRead 		time.Duration	// timeout between packages
}

func NewFrame(read Read, close Close, name Name, packageTimeout time.Duration, absoluteTimeout time.Duration) *Frame {
	return &Frame{
		read: read,
		close: close,
		name: name,
		startAt: time.Now(),
		endAtAbsolute: time.Now().Add(absoluteTimeout),
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
			if !f.startAt.IsZero() && f.startAt != f.endAtAbsolute && f.endAtAbsolute.Before(time.Now()) && f.started {
				// error timeout 
				return nil, ErrTime
			}
			// none blocking
			data, err := f.read(ctx)
			// no error, new package
			if err == nil {
				f.bufferEmptyTimeout = time.Now()
				return data, nil
			}
			if errors.Is(err, ErrEmpty) {
				continue
			}
			if errors.Is(err, ErrClosed) {
				return nil, ErrClosed
			}
		}
	}
}

func (f *Frame) Close() {
	f.close()
}

func (f* Frame) Name() string {
	return f.name()
}
