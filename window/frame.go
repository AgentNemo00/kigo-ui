package window

import (
	"image"
)
type Frame struct {
	Data 		*image.RGBA
	PositionX 	int
	PositionY 	int
}