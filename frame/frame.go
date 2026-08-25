package frame

import (
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

type Frame struct {
	read func() ([]byte, error)
	close func()
	name func() string
	
	started 			bool
	startAt 			time.Time
	endAt 				time.Time
	bufferEmptyTimeout 	time.Time
	timeoutPerRead 		time.Duration
	timeoutTotal 		time.Duration
}

func (f *Frame) Read() ([]byte, error) {
	data, err := f.read()
	if err == nil {
		f.bufferEmptyTimeout = time.Now()
		if !f.started {
			f.started = true
			f.endAt = f.startAt.Add(f.timeoutTotal)
		}
		return data, nil
	}
	if f.timeoutPerRead != 0 && f.bufferEmptyTimeout.Add(f.timeoutPerRead).Before(time.Now()) && f.started {
		return nil, ErrTimeout
	}
	if !f.startAt.IsZero() && f.startAt != f.endAt && f.endAt.Before(time.Now()) && f.started {
		// error timeout 
		return nil, ErrTime
	}
	if errors.Is(err, ErrEmpty) {
		return nil, ErrEmpty
	}
	return nil, err
}

func (f *Frame) Close() {
	f.close()
}

func (f* Frame) Name() string {
	return f.name()
}
 