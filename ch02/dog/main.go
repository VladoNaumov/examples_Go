package main

import (
	"math/rand"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	width  = 21
	height = 10
)

type game struct {
	field            [height][width]rune
	playerX, playerY int
	appleX, appleY   int
}

func newGame() *game {
	g := &game{
		playerX: 10,
		playerY: 6,
		appleX:  4,
		appleY:  3,
	}
	g.initField()
	return g
}

func (g *game) initField() {
	for j := 0; j < width; j++ {
		g.field[0][j] = '#'
		g.field[height-1][j] = '#'
	}
	for i := 1; i < height-1; i++ {
		for j := 0; j < width; j++ {
			if j == 0 || j == width-1 {
				g.field[i][j] = '#'
			} else {
				g.field[i][j] = ' '
			}
		}
	}
}

func (g *game) render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	g.field[g.playerY][g.playerX] = '@'
	g.field[g.appleY][g.appleX] = 'a'
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			termbox.SetCell(j, i, g.field[i][j], termbox.ColorWhite, termbox.ColorDefault)
		}
	}
	termbox.Flush()
}

func (g *game) movePlayer(dx, dy int) {
	newX, newY := g.playerX+dx, g.playerY+dy
	if g.field[newY][newX] != '#' {
		g.playerX, g.playerY = newX, newY
	}
	if g.playerX == g.appleX && g.playerY == g.appleY {
		g.appleX, g.appleY = 2, 7 // Временные фиксированные координаты
		// g.appleX, g.appleY = rand.Intn(width-2)+1, rand.Intn(height-2)+1
	}
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	rand.Seed(time.Now().UnixNano())
	g := newGame()

	// Канал для обработки ввода
	events := make(chan termbox.Event)
	quit := make(chan struct{})

	// Горутина для обработки ввода
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

	// Основной цикл
	for {
		g.initField() // Переинициализация поля
		g.render()

		select {
		case ev := <-events:
			if ev.Type != termbox.EventKey {
				continue
			}
			switch ev.Ch {
			case 'w':
				g.movePlayer(0, -1)
			case 's':
				g.movePlayer(0, 1)
			case 'a':
				g.movePlayer(-1, 0)
			case 'd':
				g.movePlayer(1, 0)
			case 'e':
				close(quit)
				return
			}
		default:
			// Ничего не делаем, чтобы избежать блокировки
		}
	}
}
