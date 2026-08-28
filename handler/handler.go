package handler

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	errcore "github.com/AgentNemo00/kigo-core/errors"
	"github.com/AgentNemo00/kigo-core/information"
	"github.com/AgentNemo00/kigo-core/inquiry"
	"github.com/AgentNemo00/kigo-core/notification"
	"github.com/AgentNemo00/kigo-core/order"
	"github.com/AgentNemo00/kigo-core/ui"
	"github.com/AgentNemo00/kigo-core/update"
	"github.com/AgentNemo00/kigo-ui/frame"
	"github.com/AgentNemo00/kigo-ui/paint"
	"github.com/AgentNemo00/kigo-ui/pubsub"
	"github.com/AgentNemo00/kigo-ui/window"
	"github.com/AgentNemo00/sca-instruments/log"
	ps "github.com/AgentNemo00/sca-instruments/pubsub"
	"github.com/AgentNemo00/sca-instruments/security"
)

const(
	headerSize = 16
)

type Handler struct {
	communication 		*pubsub.Communication
	config 				*Config
	channelIPC 			*frame.IPC
	channelPubSub		*frame.PubSub
	ctx 				context.Context

	pkgChan           	chan paint.Package
	window 				*window.Window
}

func NewHandler(config *Config, window *window.Window) (*Handler, error) {
	communication, err := pubsub.NewCommunication(config.PubSubUrl)
	if err != nil {
		return nil, err
	}
	var channelIPC *frame.IPC
	if config.IPCPath != "" {
		channelIPC, err = frame.NewIPC(config.IPCPath)
		if err != nil {
			return nil, err
		}		
	}
	channelPubSub, err := frame.NewPubSub(config.PubSubUrl)
	if err != nil {
		return nil, err
	}
	return &Handler{
		communication: 	communication,
		config: 		config,
		channelIPC: 	channelIPC,
		channelPubSub: 	channelPubSub,
		window: 		window,
		pkgChan: 		make(chan paint.Package),
	}, nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.ctx = ctx
	go h.Draw(ctx, h.window, h.pkgChan)
	subscription, err := h.communication.Sub.Subscribe(ctx, h.config.Name, func(ctx context.Context, metadata ps.Metadata, data *notification.Notification) {
		if metadata.Error != nil {
			log.Ctx(ctx).Error("received error in message: %v", metadata.Error)
			return
		}
		if data.From == "" {
			log.Ctx(ctx).Error("no receiver name given in message: %v", data)
			return
		}
		log.Ctx(ctx).Debug("Got message %v from %s", data, data.From)
		switch(data.Notification) {
			case inquiry.InquiryRender:
				var payload inquiry.InquiryRenderPayload
				if !h.toStruct(ctx, data, &payload) {
					return
				}
				// check if configuration is conform to the received payload. If not, send an error message back to the sender
				if !h.IsConfigConform(ctx, payload) {
					log.Ctx(ctx).Warn("Received configuration is not adaptable: %v", payload)
					h.Error(ctx, data.From, errcore.NotificationPayloadInvalid)
					return 
				}
				// send a heartbeat
				go h.Heartbeat(h.ctx, data.From)
				// start render handshake
				err := h.StartRenderHandshake(h.ctx, data.From, payload)
				if err != nil {
					log.Ctx(ctx).Err(err)
				}
			case inquiry.InquiryInformation:
				var payload inquiry.InquiryInformationPayload
				if !h.toStruct(ctx, data, &payload) {
					return
				}
				go h.Heartbeat(h.ctx, data.From)
				log.Ctx(ctx).Debug("inquiry payload: %v", payload)
				switch (payload.Type) {
					case information.UI:
						log.Ctx(ctx).Debug("inquiry ui information")
						err := h.communication.PubModule.Publish(ctx, data.From, order.Order{
							From: h.config.Name,
							To: data.From,
							Order: order.OrderInformation,
							Payload: information.UIPayload{
								Channels: h.config.Channels,
								Formats: h.config.Formats,
							},
						})
						if err != nil {
							log.Ctx(ctx).Err(err)
						}
						log.Ctx(ctx).Debug("send ui information")
					case information.Screen:
						log.Ctx(ctx).Debug("inquiry screen information")
						width, height := h.window.Size()
						err := h.communication.PubModule.Publish(ctx, data.From, order.Order{
							From: h.config.Name,
							To: data.From,
							Order: order.OrderInformation,
							Payload: information.ScreenPayload{
								Width: width,
								Height: height,
								MaxFPS: h.config.FPS,
							},
						})
						if err != nil {
							log.Ctx(ctx).Err(err)
						}
						log.Ctx(ctx).Debug("send screen information")
					case information.Point:
						payload := information.PointPayload{}
						if !h.toStruct(ctx, data, &payload) {
							return
						}
						log.Ctx(ctx).Debug("inquiry position information with payload: %v", payload)
						isOccupied := h.window.IsPointOccupied(payload.X, payload.Y)
						if !isOccupied {
							payload.X = -1
							payload.Y = -1
						}
						err := h.communication.PubModule.Publish(ctx, data.From, order.Order{
							From: h.config.Name,
							To: data.From,
							Order: order.OrderInformation,
							Payload: information.OverlapingResponse{
								X:   payload.X,
								Y:    payload.Y,
								Width:  -1,
								Height: -1,
							},
						})
						if err != nil {
							log.Ctx(ctx).Err(err)
						}
					case information.Area:
						payload := information.AreaPayload{}
						if !h.toStruct(ctx, data, &payload) {
							return
						}
						log.Ctx(ctx).Debug("inquiry area information with payload: %v", payload)
						x, y, width, height := h.window.IsAreaOccupied(payload.X, payload.Y, payload.Width, payload.Height)
						err := h.communication.PubModule.Publish(ctx, data.From, order.Order{
							From: h.config.Name,
							To: data.From,
							Order: order.OrderInformation,
							Payload: information.OverlapingResponse{
								X:   x,
								Y:    y,
								Width:  width,
								Height: height,
							},
						})
						if err != nil {
							log.Ctx(ctx).Err(err)
						}
						log.Ctx(ctx).Debug("send area information with response: %d, %d, %d, %d", x, y, width, height)
					default:
						h.Error(ctx, data.From, errcore.NotificationTypeInvalid)
				}
			default:
				h.Error(ctx, data.From, errcore.NotificationTypeInvalid)
		}
	})
	if err != nil {
		return err
	}
	h.communication.Subscription = subscription
	return h.window.Start(ctx)
}

func (h *Handler) StartRenderHandshake(ctx context.Context, from string, payload inquiry.InquiryRenderPayload) error {
	// create channel name for the transmission
	name, err := security.UUID()
	if err != nil {
		h.Error(ctx, from, errcore.Internal)
		return err
	}

	ctxTransmission, cancel := context.WithCancel(ctx)
	dataChan := make(chan Data)
	var channel *frame.Frame
	channelClose := func ()  {
		log.Ctx(ctx).Info("channel closed")
		channel.Close()
		close(dataChan)
		cancel()
	}
	frameSize := payload.MaxFrameSize
	if frameSize <= 0 {
		// no frame size set. maximal possible frame size set
		width, height := h.window.Size()
		frameSize = width * height + 8 // add header variables
	}
	objID := uint32(payload.ObjectID)
	if objID == 0 {
		objID = h.window.EnsureID()
	}
	// create channel for the transmission
	switch(payload.Channel) {
		case ui.IPC:
			log.Ctx(ctx).Debug("choose channel ipc")
			channel, err = h.channelIPC.Open(ctxTransmission, name, (headerSize+frameSize)*h.config.OverallPackageBuffer, payload.Timeout, payload.Time)
			if err != nil {
				h.Error(ctx, from, errcore.Channel)
				log.Ctx(ctx).Error("ipc could not be open")
				return err
			}
			log.Ctx(ctx).Info("IPC channel open with name: %s", name)
		case ui.PubSub:
			log.Ctx(ctx).Debug("choose channel pubsub")
			channel, err = h.channelPubSub.Open(ctxTransmission, name, payload.Timeout, payload.Time)
			if err != nil {
				h.Error(ctx, from, errcore.Channel)
				log.Ctx(ctx).Error("ipc could not be open")
				return err
			}
			log.Ctx(ctx).Info("PubSub channel open with name: %s", name)
		default:
			h.Error(ctx, from, errcore.Unsupported)
			return fmt.Errorf("not supported channel")
	}
	width, height := h.window.Size()
	err = h.communication.PubModule.Publish(ctx, from, order.Order{
		From: h.config.Name,
		To: from,
		Order: order.OrderRender,
		Payload: order.OrderRenderPayload{
			ScreenWidth: width,
			ScreenHeight: height,
			MaxFrameSize: frameSize,
			ChannelName: channel.Name(),
			ObjectID: int(objID),
		},
	})
	if err != nil {
		log.Ctx(ctx).Err(err)
		return err
	}
	go h.Transform(ctxTransmission, dataChan, payload.Format, h.pkgChan)
	go h.Transmission(ctxTransmission, dataChan, channel, payload, channelClose)
	return nil
}

func (h *Handler) Transmission(ctx context.Context, dataChan chan Data, frames *frame.Frame, payload inquiry.InquiryRenderPayload, close func()) {
	estimatedWaitingTime := time.Duration(0)
	if payload.FPS != 0 {
		estimatedWaitingTime = time.Duration(time.Millisecond*time.Duration(1000/payload.FPS))
	}
	Nctx, cancel := context.WithCancel(ctx)
	for {
		select {
			case <- Nctx.Done():
				cancel()
				close()
				return
			default:
				//blocking until data or timeout
				data, err := frames.Read(Nctx)
				if err != nil {
					cancel()
					close()
					log.Ctx(ctx).Err(err)
					return
				}
				if data == nil {
					continue
				}
				log.Ctx(ctx).Debug("got data to transmite from %s with a length of %d", frames.Name(), len(data))
				dataChan <- Data{
					Data: data,
				}
				time.Sleep(estimatedWaitingTime)
		}
	}
}

func (h *Handler) Transform(ctx context.Context, dataChan chan Data, format string, packageChan chan paint.Package) {
	for {
		select {
			case <- ctx.Done():
				return
			case dataPackage, ok := <- dataChan:
				if !ok {
					log.Ctx(ctx).Error("channel transform closed")
					return
				}
				if dataPackage.Data == nil {
					log.Ctx(ctx).Error("transform data empty")
					return
				}
				id := binary.BigEndian.Uint32(dataPackage.Data[0:4])
				log.Ctx(ctx).Debug("got data to transform from %d", id)
				positionX := binary.BigEndian.Uint16(dataPackage.Data[4:6])
				positionY := binary.BigEndian.Uint16(dataPackage.Data[6:8])
				width := binary.BigEndian.Uint16(dataPackage.Data[8:10])
				height := binary.BigEndian.Uint16(dataPackage.Data[10:12])
				size := binary.BigEndian.Uint32(dataPackage.Data[12:16])
				if size <= 0 {
					log.Ctx(ctx).Debug("received empty data package ")			 
					packageChan <- paint.Package{
						ID: 		id,	
						PositionX: 	int(positionX),
						PositionY: 	int(positionY),
						Width: 		int(width),
						Height: 	int(height),
						Data: 		make([]byte, 0),
					}
					return
				}
				data := dataPackage.Data[headerSize:headerSize+size]
				if format != ui.RAW {
					// decode data
					switch(format) {
						case ui.PNG:
							dataDecoded, err := frame.DecodePNG(ctx, data)
							if err != nil {
								log.Ctx(ctx).Err(err)
								continue
							}
							data = dataDecoded
						case ui.JPEG:
							dataDecoded, err := frame.DecodeJPEG(ctx, data)
							if err != nil {
								log.Ctx(ctx).Err(err)
								continue
							}
							data = dataDecoded
						default:
							log.Ctx(ctx).Error("unsupported format: %s", format)
							continue
					}
				}	
				log.Ctx(ctx).Debug("received data package %d, on position %d, %d and dimensions %d, %d", id, positionX, positionY, width, height)			 
				packageChan <- paint.Package{
					ID: 		id,	
					PositionX: 	int(positionX),
					PositionY: 	int(positionY),
					Width: 		int(width),
					Height: 	int(height),
					Data: 		data,
				}
			default:
		}
	}
}

func (h *Handler) Draw(ctx context.Context, window *window.Window, packageChan chan paint.Package) error {
	// This does not allow different FPS settings. Request a drawing ID in the UI handshake
	for {
		select {
			case <- ctx.Done():
				return ctx.Err()
			case pkg, ok := <- packageChan:
				if !ok {
					return nil
				}
				log.Ctx(ctx).Debug("got data to draw from %d", pkg.ID)
				if len(pkg.Data) == 0 {
					log.Ctx(ctx).Debug("Remove id %d", pkg.ID)
					window.Remove(pkg.ID)
				} else {
					err := window.Add(pkg)
					if err != nil {
						log.Ctx(ctx).Err(err)
						return err
					} 
					log.Ctx(ctx).Debug("Add id %d", pkg.ID)
				}
				window.Draw()
			}
		}
}


func (h *Handler) IsConfigConform(ctx context.Context, payload inquiry.InquiryRenderPayload) bool {
	if h.config.FPS < payload.FPS {
		return false
	} 
	if slices.Index(h.config.Channels, payload.Channel) == -1 {
		return false
	}
	if slices.Index(h.config.Formats, payload.Format) == -1 {
		return false
	}
	return true
}

func (h *Handler) Heartbeat(ctx context.Context, to string) {
	log.Ctx(ctx).Debug("send heartbeat for %s", to)
	h.communication.PubKigo.Publish(ctx, h.config.KiGo, notification.Notification{
		From: h.config.Name,
		To: h.config.KiGo,
		Notification: notification.NotificationUpdate,
		Payload: notification.NotificationUpdatePayload{
			Type: update.Heartbeat,
			Payload: to,
		},
	})
}

func (h *Handler) Error(ctx context.Context, to string, errorCore int) {
	msg := order.Order{
		From: h.config.Name,
		To: to,	
		Order: order.OrderError,
		Payload: order.OrderErrorPayload{
			Message: fmt.Sprintf("%d", errorCore),
		},
	}
	err := h.communication.PubModule.Publish(ctx, to, msg)
	if err != nil {
		log.Ctx(ctx).Err(err)
	}
}

func (h *Handler) Stop(ctx context.Context) {
	h.window.Stop()
	h.communication.Subscription.Unsubscribe(ctx)
}

func (h *Handler) toStruct(ctx context.Context,m *notification.Notification, out any) bool {
	err := mapToStruct(m.Payload, &out)
	if err != nil {
		log.Ctx(ctx).Err(err)
		h.Error(ctx, m.From, errcore.NotificationPayloadInvalid)
		return false
	}
	return true
}

func mapToStruct(m any, out any) error {
    data, err := json.Marshal(m)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, out)
}
