package main

import (
	"math/rand"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	width  = 20 // ширина поля
	height = 20 // высота поля
)

type game struct {
	snake  [][2]int            // координаты сегментов змейки [y][x]
	food   [2]int              // координаты еды [y][x]
	dir    [2]int              // направление [dy][dx]
	field  [height][width]rune // игровое поле
	length int                 // длина змейки
}

// newGame создаёт новую игру
func newGame() *game {
	g := &game{
		snake:  [][2]int{{height / 2, width / 2}}, // змейка начинается в центре
		dir:    [2]int{0, 1},                      // начальное направление: вправо
		length: 1,
	}
	g.food = [2]int{rand.Intn(height), rand.Intn(width)} // случайная еда
	g.initField()
	return g
}

// initField заполняет поле точками
func (g *game) initField() {
	for y := range g.field {
		for x := range g.field[y] {
			g.field[y][x] = '.'
		}
	}
}

// render отображает поле, змейку и еду
func (g *game) render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			termbox.SetCell(x, y, g.field[y][x], termbox.ColorWhite, termbox.ColorDefault)
		}
	}
	for _, seg := range g.snake {
		termbox.SetCell(seg[1], seg[0], '@', termbox.ColorWhite, termbox.ColorDefault)
	}
	termbox.SetCell(g.food[1], g.food[0], '*', termbox.ColorWhite, termbox.ColorDefault)
	termbox.Flush()
}

// move двигает змейку, проверяет столкновения и еду
func (g *game) move() bool {
	// Новая голова
	head := [2]int{g.snake[0][0] + g.dir[0], g.snake[0][1] + g.dir[1]}

	// Проверка выхода за границы
	if head[0] < 0 || head[0] >= height || head[1] < 0 || head[1] >= width {
		return false
	}

	// Проверка столкновения с собой
	for _, seg := range g.snake {
		if head[0] == seg[0] && head[1] == seg[1] {
			return false
		}
	}

	// Добавляем новую голову
	g.snake = append([][2]int{head}, g.snake...)

	// Если съели еду
	if head[0] == g.food[0] && head[1] == g.food[1] {
		g.length++
		// Новая случайная еда
		g.food = [2]int{rand.Intn(height), rand.Intn(width)}
	} else {
		// Удаляем хвост, если не съели еду
		g.snake = g.snake[:g.length]
	}
	return true
}

func main() {
	// Инициализация терминала
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	rand.Seed(time.Now().UnixNano())
	g := newGame()

	events := make(chan termbox.Event)
	quit := make(chan struct{})
	ticker := time.NewTicker(100 * time.Millisecond) // обновление каждые 100 мс
	defer ticker.Stop()

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
		select {
		case ev := <-events:
			if ev.Type == termbox.EventKey {
				switch ev.Ch {
				case 'w': // вверх
					if g.dir != [2]int{1, 0} { // запрещаем разворот вниз
						g.dir = [2]int{-1, 0}
					}
				case 's': // вниз
					if g.dir != [2]int{-1, 0} { // запрещаем разворот вверх
						g.dir = [2]int{1, 0}
					}
				case 'a': // влево
					if g.dir != [2]int{0, 1} { // запрещаем разворот вправо
						g.dir = [2]int{0, -1}
					}
				case 'd': // вправо
					if g.dir != [2]int{0, -1} { // запрещаем разворот влево
						g.dir = [2]int{0, 1}
					}
				case 'q': // выход
					close(quit)
					return
				}
			}
		case <-ticker.C:
			g.initField()
			if !g.move() { // если столкновение, выход
				close(quit)
				return
			}
			g.render()
		}
	}
}
