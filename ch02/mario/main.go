package main

import (
	"math"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	mapWidth  = 80
	mapHeight = 25
)

type game struct {
	field [mapHeight][mapWidth + 1]rune
	mario TObject
}

type TObject struct {
	x, y      float32
	width     float32
	height    float32
	vertSpeed float32
}

func newGame() *game {
	g := &game{}
	g.mario = TObject{x: 39, y: 10, width: 3, height: 3, vertSpeed: 0}
	g.clearMap()
	return g
}

func (g *game) clearMap() {
	for i := 0; i < mapWidth; i++ {
		g.field[0][i] = '.'
	}
	g.field[0][mapWidth] = '\x00'
	for j := 1; j < mapHeight; j++ {
		for i := 0; i <= mapWidth; i++ {
			g.field[j][i] = g.field[0][i]
		}
	}
}

func (g *game) render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for j := 0; j < mapHeight; j++ {
		for i := 0; i < mapWidth; i++ {
			termbox.SetCell(i, j, g.field[j][i], termbox.ColorWhite, termbox.ColorDefault)
		}
	}
	termbox.Flush()
}

func (g *game) moveMario() {
	g.mario.vertSpeed += 0.05
	g.mario.y += g.mario.vertSpeed
}

func (g *game) putMario() {
	ix := int(math.Round(float64(g.mario.x)))
	iy := int(math.Round(float64(g.mario.y)))
	iWidth := int(math.Round(float64(g.mario.width)))
	iHeight := int(math.Round(float64(g.mario.height)))

	for i := ix; i < ix+iWidth; i++ {
		for j := iy; j < iy+iHeight; j++ {
			if j >= 0 && j < mapHeight && i >= 0 && i < mapWidth {
				g.field[j][i] = '@'
			}
		}
	}
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	g := newGame()
	events := make(chan termbox.Event)
	quit := make(chan struct{})
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				events <- termbox.PollEvent()
			}
		}
	}()

	for {
		select {
		case ev := <-events:
			if ev.Type == termbox.EventKey && ev.Key == termbox.KeySpace {
				close(quit)
				return
			}
		case <-ticker.C:
			g.clearMap()
			g.moveMario()
			g.putMario()
			g.render()
		}
	}
}
