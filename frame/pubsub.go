package frame

import (
	"context"
	"time"

	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/AgentNemo00/sca-instruments/pubsub"
	"github.com/AgentNemo00/sca-instruments/pubsub/nats"
)


type PubSub struct {
	sub pubsub.Subscriber[[]byte]
}

func NewPubSub(url string) (*PubSub, error) {
	sub, err := nats.SubscriberWithURL[[]byte](url)
	if err != nil {
		return nil, err
	}
	return &PubSub{
		sub: sub,
	}, nil
}

func (p *PubSub) Open(ctx context.Context, name string, timeoutPerRead time.Duration, timeoutTotal time.Duration) (*Frame, error) {
	frameChan := make(chan []byte)
	frameErr := make(chan error)
	frameHandler := &Frame{
		started: false,
		startAt: time.Now(),
		endAtAbsolute: time.Now(),
		bufferEmptyTimeout: time.Now(),
		timeoutPerRead: timeoutPerRead,
		read: func(Nctx context.Context) ([]byte, error) {
			select{
			case <- Nctx.Done():
				return nil, Nctx.Err()
			case data, ok := <- frameChan:
				if !ok {
					return nil, ErrClosed
				}
				return data, nil
			case data, ok := <- frameErr:
				if !ok {
					return nil, ErrClosed
				}
				return nil, data
			default:
				return nil, ErrEmpty
			}
		},
		name: func () string {
			return name
		},
	}
	subscription, err := p.sub.Subscribe(ctx, name, func(ctx context.Context, metadata pubsub.Metadata, data *[]byte) {
		if metadata.Error != nil {
			log.Ctx(ctx).Error("received error in message: %v", metadata.Error)
			frameErr <- metadata.Error
			return
		}
		frameChan <- *data
	} )
	if err != nil {
		return nil, err
	}
	frameHandler.close = func ()  {
		subscription.Unsubscribe(ctx)
		close(frameChan)
		close(frameErr)
	}
	return frameHandler, nil
}
