package main

import "fmt"

// пустой интерфейс
type Empty interface{}

func print_value(value Empty) {
	fmt.Println(value)
}

type person struct {
	name string
}

type account struct {
	id int
}

func main() {

	tom := person{"Tom"} // ✅ объявляем переменную tom

	tom_acc := account{125}

	print_value(tom)     // {Tom}
	print_value(tom_acc) // {125}
}
