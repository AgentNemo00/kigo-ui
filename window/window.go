package window

import (
	"context"
	"image"
	"math"
	"math/rand"
	"sync"

	"github.com/AgentNemo00/kigo-ui/paint"
	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
)

type Window struct {
	app 	*gogpu.App
	mu 		*sync.Mutex
	frames 	*map[uint32]Frame
	saved   []uint32
	ticksEnsuranceSaved uint32
}

func NewWindow(ctx context.Context) (*Window ,error) {
	w := &Window{saved: make([]uint32, 0), ticksEnsuranceSaved: rand.Uint32()}
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(" ").
		WithFullscreen(). // TODO: fullscreen not working on macOS, need to investigate
		WithFrameless(true).
		WithContinuousRender(false))
	var canvas *ggcanvas.Canvas
	mu := new(sync.Mutex)
	frames := make(map[uint32]Frame, 0)
	w.app = app
	w.frames = &frames
	w.mu = mu

	app.OnDraw(func(dc *gogpu.Context) {
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}

		// Lazy canvas init.
		if canvas == nil {
			provider := app.GPUContextProvider()
			if provider == nil {
				return
			}
			var err error
			canvas, err = ggcanvas.New(provider, w, h)
			if err != nil {
				log.Ctx(ctx).Err(err)
				return
			}
		}

		cc := canvas.Context()

		// Dark background.
		cc.SetRGBA(0.08, 0.08, 0.12, 1)
		cc.Clear()

		mu.Lock()
		log.Ctx(ctx).Debug("rendering %d frames", len(frames))
		for _, frame := range frames {
			if frame.Data == nil {
				continue
			}
			buf := gg.ImageBufFromImage(frame.Data)
			cc.DrawImage(buf, float64(frame.PositionX), float64(frame.PositionY))
		}
		mu.Unlock()

		// Present: upload pixmap to GPU texture and blit to surface.
		canvas.MarkDirty()
		rt := dc.RenderTarget()
		if err := canvas.Render(rt); err != nil {
			log.Ctx(ctx).Err(err)
		}
	})

	w.app.OnClose(func() {
		gg.CloseAccelerator()
		if canvas != nil {
			err := canvas.Close()
			if err != nil {
				log.Ctx(ctx).Err(err)
			}
		}
	})
	return w, nil
}

func (w *Window) Stop() {
	w.app.Close()
}

func (w *Window) Start(ctx context.Context) error {
	return w.app.Run()
}

func (w *Window) Draw() {
	defer w.ResetEnsurance()
	w.app.RequestRedraw()
}

func (w *Window) Add(pkg paint.Package) error {
	img := &image.RGBA{
		Pix:    pkg.Data,
		Stride: 4 * pkg.Width,
		Rect:   image.Rect(0, 0, pkg.Width, pkg.Height),
	}

	w.mu.Lock()
	defer w.ResetEnsurance()
	defer w.mu.Unlock()
	(*w.frames)[pkg.ID] = Frame{
		Data: img,
		PositionX: pkg.PositionX,
		PositionY: pkg.PositionY,
	}
	w.ClearEnsurance(pkg.ID)
	
	return nil
}

func (w *Window) IsPointOccupied(x, y int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, frame := range *w.frames {
		if x > frame.PositionX && x < frame.PositionX+frame.Data.Bounds().Dx() &&
			y > frame.PositionY && y < frame.PositionY+frame.Data.Bounds().Dy() {
			return true
		}
	}
	return false
}

func (w *Window) IsAreaOccupied(x, y, width, height int) (int, int, int, int) { 
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, frame := range *w.frames {
		// check if overlapping
		if x < frame.PositionX + frame.Data.Bounds().Dx() &&
		x + width > frame.PositionX &&
		y < frame.PositionY + frame.Data.Bounds().Dy() &&
		y + height > frame.PositionY{
				left := math.Max(float64(x), float64(frame.PositionX))
				top := math.Max(float64(y), float64(frame.PositionY))
				right := math.Min(float64(x+width), float64(frame.PositionX+frame.Data.Bounds().Dx()))
				bottom := math.Min(float64(y+height), float64(frame.PositionY+frame.Data.Bounds().Dy()))
			return int(left), int(top), int(right - left), int(bottom - top)
		}
	}
	return -1, -1, -1, -1
}

func (w *Window) Remove(id uint32) {
	defer w.ResetEnsurance()
	delete(*w.frames, id)
}

func (w *Window) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for k := range *w.frames {
		delete(*w.frames, k)
	}
}

func (w *Window) EnsureID() uint32 {
	w.mu.Lock()
	defer w.ResetEnsurance()
	defer w.mu.Unlock()
	l: for {
		id := rand.Uint32()
		for k := range *w.frames {
			if k == id {
				continue l
			}
		}
		for _, k := range w.saved {
			if k == id {
				continue l
			}
		}
		w.saved = append(w.saved, id)
		return id
	}
}

func (w *Window) ResetEnsurance() {
	w.ticksEnsuranceSaved--
	if w.ticksEnsuranceSaved <= 0 {
		w.mu.Lock()
		w.saved = make([]uint32, 0)
		w.mu.Unlock()
	}
	w.ticksEnsuranceSaved = rand.Uint32()
}

func (w *Window) ClearEnsurance(id uint32) {
	for i, k := range w.saved {
		if id == k {
			w.saved[i] = w.saved[len(w.saved)-1]
            w.saved =  w.saved[:len(w.saved)-1]
			return
		}
	}
}

func (w *Window) Size() (int, int) {
	return w.app.Size()
}