package pubsub

import (
	"github.com/AgentNemo00/sca-instruments/pubsub"
	"github.com/AgentNemo00/sca-instruments/pubsub/nats"
	"github.com/AgentNemo00/kigo-core/order"
	"github.com/AgentNemo00/kigo-core/notification"
)

type Communication struct {
	PubModule pubsub.Publisher[order.Order]
	PubKigo pubsub.Publisher[notification.Notification]
	Sub pubsub.Subscriber[notification.Notification]
	Subscription pubsub.Subscription
}

func NewCommunication(url string) (*Communication, error) {
	pubModule, err := nats.PublisherWithURL[order.Order](url)
	if err != nil {
		return nil, err
	}
	pubKigo, err := nats.PublisherWithURL[notification.Notification](url)
	if err != nil {
		return nil, err
	}
	sub, err := nats.SubscriberWithURL[notification.Notification](url)
	if err != nil {
		return nil, err
	}
	return &Communication{
		PubModule: pubModule,
		PubKigo: pubKigo,
		Sub: sub,
	}, nil
}
