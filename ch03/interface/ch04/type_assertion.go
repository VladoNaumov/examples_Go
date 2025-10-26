package main

/*
Type assertion (подтверждение типа)
В языке Go есть механизм type assertion - специальная конструкция, которая проверяет, содержит ли значение интерфейса указанный тип или нет, то есть выполняет подтверждение типа. Если значение интерфейса содержит данный конкретный тип, то возвращается true и значение данного конкретного типа; иначе возвращается false и ноль в качестве значения
*/

import "fmt"

// интерфейс перемещения объекта
type Movable interface {
	move()
}

type Rectangle struct {
	x, // X-координата левого верхнего угла
	y, // Y-координата левого верхнего угла
	width, // ширина
	height int // высота
}

type Circle struct {
	x, // X-координата центра круга
	y, // Y-координата центра круга
	radius int // радиус круга
}

// реализация интерфейса Movable для Rectangle
func (r Rectangle) move() {
	fmt.Println("Перемещаем прямоугольник")
}

// реализация интерфейса Movable для Circle
func (c Circle) move() {
	fmt.Println("Перемещаем круг")
}

func main() {

	var shape Movable = Rectangle{x: 20, y: 10, width: 150, height: 100}
	//move_object(shape)

	// проверяем, реализует ли структура Rectangle интерфейс Movable
	value, ok := shape.(Rectangle)
	fmt.Println(ok)    // true
	fmt.Println(value) // {20 10 150 100}
}
