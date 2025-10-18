package main

import (
	"math/rand"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	h = 25
	w = 80
)

type field [h][w]rune

func newField() *field {
	var f field
	for i := range f {
		for j := range f[i] {
			f[i][j] = ' '
		}
	}
	return &f
}

func (f *field) newSnow() {
	for i := 0; i < w; i++ {
		if rand.Intn(12) == 1 {
			f[0][i] = '*'
		} else {
			f[0][i] = ' '
		}
	}
}

func (f *field) moveSnow() {
	for j := h - 1; j >= 0; j-- {
		for i := 0; i < w; i++ {
			if f[j][i] == '*' {
				f[j][i] = ' '
				dx := i + 1
				if rand.Intn(10) < 1 {
					dx++
				}
				if rand.Intn(10) < 1 {
					dx--
				}
				if dx >= 0 && dx < w && j+1 < h {
					f[j+1][dx] = '*'
				}
			}
		}
	}
}

func (f *field) render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for i := 0; i < h; i++ {
		for j := 0; j < w; j++ {
			termbox.SetCell(j, i, f[i][j], termbox.ColorWhite, termbox.ColorDefault)
		}
	}
	termbox.Flush()
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	rand.Seed(time.Now().UnixNano())
	f := newField()

	// Канал для обработки выхода по пробелу
	quit := make(chan struct{})

	// Горутина для обработки ввода
	go func() {
		for {
			if ev := termbox.PollEvent(); ev.Type == termbox.EventKey && ev.Key == termbox.KeySpace {
				close(quit)
				return
			}
		}
	}()

	// Основной цикл анимации
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			f.moveSnow()
			f.newSnow()
			f.render()
		}
	}
}
