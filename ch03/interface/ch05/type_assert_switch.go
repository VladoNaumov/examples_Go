package main

/*

Проверка типа и Type switch
Мы можем использовать подряд несколько подтверждений типа, используя конструкцию Type switch.
Эта конструкция использует оператор switch, сравнивая тип значения.
Для проверки типа применяются операторы case, после которых указывается проверяемый тип,
которому должно соответствовать значение интерфейса. Стоит помнить, что если переменной интерфейса не присвовано значение,
то оно равно nil, а его тип тоже nil.

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

type Point struct {
	x, // X-координата
	y int // Y-координата
}

// функция проверки типа
func check(i interface{}) {

	switch value := i.(type) {

	case Rectangle:
		fmt.Println("Type: Rectangle. Value: ", value)

	case Circle:
		fmt.Println("Type: Circle. Value: ", value)

	default:
		fmt.Println("Type: Undefined")
	}
}

func main() {

	check(Rectangle{x: 20, y: 10, width: 150, height: 100})
	check(Circle{x: 80, y: 50, radius: 70})
	check(Point{x: 20, y: 10})
}
