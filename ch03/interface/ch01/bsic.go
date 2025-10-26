package main

// interface
import "fmt"

type Vehicle interface {
	move()
}

// структура "Автомобиль"
type Car struct{}

// структура "Самолет"
type Aircraft struct{}

func (c Car) move() {
	fmt.Println("Автомобиль едет")
}
func (a Aircraft) move() {
	fmt.Println("Самолет летит")
}

func main() {

	var tesla Vehicle = Car{}
	var boing Vehicle = Aircraft{}
	tesla.move() // Автомобиль едет
	boing.move() // Самолет летит
}
