package handler

import (
	"github.com/AgentNemo00/kigo-core/ui"
)

type Config struct {
	Name 					string
	Formats 				[]string
	Channels 				[]string
	FPS 					int
	PubSubUrl 				string
	KiGo 					string
	IPCPath 				string
	OverallPackageBuffer 	int      // recommended minimum the amounts of modules
}

func (c *Config) Default() {
	if c.Name == "" {
		c.Name = "KiGoUI"
	}
	if c.KiGo == "" {
		c.KiGo = "KiGo"
	}
	if c.PubSubUrl == "" {
		c.PubSubUrl = "nats://127.0.0.1:4222"
	}
	if c.IPCPath == "" {
		c.IPCPath = "/tmp"
	} 
	if c.FPS == 0 {
		c.FPS = 24
	}
	if len(c.Formats) == 0 {
		c.Formats = []string{ui.RAW, ui.PNG, ui.JPEG}
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{ui.IPC, ui.PubSub}
	}
	if c.OverallPackageBuffer == 0 {
		c.OverallPackageBuffer = 3
	}
}	
