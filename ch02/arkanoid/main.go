package main

import (
	"fmt"
	"math"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	width  = 65
	height = 25
)

type game struct {
	field     [height][width + 1]rune
	racket    TRacket
	ball      TBall
	hitCnt    int
	maxHitCnt int
	lvl       int
	running   bool
}

type TRacket struct {
	x, y, w int
}

type TBall struct {
	x, y   float32
	ix, iy int
	alfa   float32
	speed  float32
}

func newGame() *game {
	g := &game{
		racket: TRacket{w: 7, x: (width - 7) / 2, y: height - 1},
		ball:   TBall{x: 2, y: 2, alfa: -1, speed: 0.5},
		lvl:    1,
	}
	g.ball.ix = int(math.Round(float64(g.ball.x)))
	g.ball.iy = int(math.Round(float64(g.ball.y)))
	g.initField()
	return g
}

func (g *game) initField() {
	for i := 0; i < width; i++ {
		g.field[0][i] = '#'
	}
	g.field[0][width] = '\x00'

	for i := 0; i < width; i++ {
		g.field[1][i] = g.field[0][i]
	}
	for i := 1; i < width-1; i++ {
		g.field[1][i] = ' '
	}

	for i := 2; i < height; i++ {
		for j := 0; j <= width; j++ {
			g.field[i][j] = g.field[1][j]
		}
	}

	if g.lvl == 2 {
		for i := 20; i < 50; i++ {
			g.field[10][i] = '#'
		}
	}

	if g.lvl == 3 {
		for j := 1; j < 10; j++ {
			for i := 1; i < 65; i += 7 {
				g.field[j][i] = '#'
			}
		}
	}
}

func (g *game) putRacket() {
	for i := g.racket.x; i < g.racket.x+g.racket.w; i++ {
		g.field[g.racket.y][i] = '@'
	}
}

func (g *game) putBall() {
	g.field[g.ball.iy][g.ball.ix] = '*'
}

func (g *game) moveBall(x, y float32) {
	g.ball.x = x
	g.ball.y = y
	g.ball.ix = int(math.Round(float64(g.ball.x)))
	g.ball.iy = int(math.Round(float64(g.ball.y)))
}

func (g *game) autoMoveBall() {
	if g.ball.alfa < 0 {
		g.ball.alfa += float32(math.Pi * 2)
	}
	if g.ball.alfa > float32(math.Pi*2) {
		g.ball.alfa -= float32(math.Pi * 2)
	}

	bl := g.ball
	g.moveBall(g.ball.x+float32(math.Cos(float64(g.ball.alfa)))*g.ball.speed,
		g.ball.y+float32(math.Sin(float64(g.ball.alfa)))*g.ball.speed)

	if g.field[g.ball.iy][g.ball.ix] == '#' || g.field[g.ball.iy][g.ball.ix] == '@' {
		if g.field[g.ball.iy][g.ball.ix] == '@' {
			g.hitCnt++
		}
		if g.ball.ix != bl.ix && g.ball.iy != bl.iy {
			if g.field[bl.iy][g.ball.ix] == g.field[g.ball.iy][bl.ix] {
				bl.alfa = bl.alfa + float32(math.Pi)
			} else {
				if g.field[bl.iy][g.ball.ix] == '#' {
					bl.alfa = (2*float32(math.Pi) - bl.alfa) + float32(math.Pi)
				} else {
					bl.alfa = (2*float32(math.Pi) - bl.alfa)
				}
			}
		} else if g.ball.iy == bl.iy {
			bl.alfa = (2*float32(math.Pi) - bl.alfa) + float32(math.Pi)
		} else {
			bl.alfa = (2*float32(math.Pi) - bl.alfa)
		}
		g.ball = bl
		g.autoMoveBall()
	}
}

func (g *game) moveRacket(dx int) {
	g.racket.x += dx
	if g.racket.x < 1 {
		g.racket.x = 1
	}
	if g.racket.x+g.racket.w >= width {
		g.racket.x = width - 1 - g.racket.w
	}
}

func (g *game) render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			termbox.SetCell(j, i, g.field[i][j], termbox.ColorWhite, termbox.ColorDefault)
		}
		if i == 1 {
			for j, c := range fmt.Sprintf("   lvl %d   ", g.lvl) {
				termbox.SetCell(width+j, i, c, termbox.ColorWhite, termbox.ColorDefault)
			}
		}
		if i == 3 {
			for j, c := range fmt.Sprintf("   hit %d   ", g.hitCnt) {
				termbox.SetCell(width+j, i, c, termbox.ColorWhite, termbox.ColorDefault)
			}
		}
		if i == 4 {
			for j, c := range fmt.Sprintf("   max %d   ", g.maxHitCnt) {
				termbox.SetCell(width+j, i, c, termbox.ColorWhite, termbox.ColorDefault)
			}
		}
	}
	termbox.Flush()
}

func (g *game) showPreview() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for j, c := range fmt.Sprintf("LEVEL %d", g.lvl) {
		termbox.SetCell(width/2-5+j, height/2, c, termbox.ColorWhite, termbox.ColorDefault)
	}
	termbox.Flush()
	time.Sleep(1 * time.Second)
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	g := newGame()
	g.showPreview()

	events := make(chan termbox.Event)
	quit := make(chan struct{})
	ticker := time.NewTicker(10 * time.Millisecond)
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
			if ev.Type == termbox.EventKey {
				switch ev.Ch {
				case 'a', 'A':
					g.moveRacket(-1)
				case 'd', 'D':
					g.moveRacket(1)
				case 'w', 'W':
					g.running = true
				}
				if ev.Key == termbox.KeyEsc {
					close(quit)
					return
				}
			}
		case <-ticker.C:
			if g.running {
				g.autoMoveBall()
			}
			if g.ball.iy > height {
				g.running = false
				if g.hitCnt > g.maxHitCnt {
					g.maxHitCnt = g.hitCnt
				}
				g.hitCnt = 0
			}
			if g.hitCnt > 10 {
				g.lvl++
				g.running = false
				g.maxHitCnt = 0
				g.hitCnt = 0
				g.showPreview()
			}

			g.initField()
			g.putRacket()
			g.putBall()
			g.render()

			if !g.running {
				g.moveBall(float32(g.racket.x+g.racket.w/2), float32(g.racket.y-1))
			}
		}
	}
}
