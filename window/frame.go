package window

import (
	"image"
	"image/draw"
)
type Frame struct {
	Data 		*image.RGBA
	PositionX 	int
	PositionY 	int
}

func (f *Frame) Blit(data *image.RGBA, positionX int, positionY int) {
	// Define the target bounds on the destination image
	offset := image.Pt(positionX, positionY)
	targetRect := image.Rectangle{
		Min: offset,
		Max: offset.Add(data.Bounds().Size()),
	}

	// Draw.Src completely overwrites pixels (ignoring alpha blending).
	// Use draw.Over if you want alpha channel blending instead.
	draw.Draw(f.Data, targetRect, data, data.Bounds().Min, draw.Src)
}