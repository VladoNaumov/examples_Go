package main

// Реализация нескольких интерфейсов
import "fmt"

// интерфейс перемещения объекта
type Movable interface {
	move()
}

// интерфейс отрисовки объекта
type Drawable interface {
	draw()
}

type Rectangle struct{}

// реализация интерфейса Movable для Rectangle
func (r Rectangle) move() {
	fmt.Println("Перемещаем прямоугольник")
}

// реализация интерфейса Drawable для Rectangle
func (r Rectangle) draw() {
	fmt.Println("Рисуем прямоугольник")
}

func move_object(obj Movable) {
	obj.move()
}

func draw_object(obj Drawable) {
	obj.draw()
}

func main() {

	rect := Rectangle{}
	move_object(rect) // Перемещаем прямоугольник
	draw_object(rect) // Рисуем прямоугольник

	// или так
	var movable1 Movable = rect
	move_object(movable1)

	var drawable1 Drawable = rect
	draw_object(drawable1)
}
