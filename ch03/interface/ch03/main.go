package main

// Реализация интерфейсов в структурах распространяется и на указатели на эти структуры.
import "fmt"

type Reader interface {
	read()
}

type File struct {
	text string
}

// реализация интерфейса Reader для File
func (f File) read() {
	fmt.Println(f.text)
}

// функция перемещения объекта Movable
func read_data(data Reader) {
	data.read()
}

func main() {

	file := File{"Hello METANIT.COM"}
	// так можно
	read_data(file)

	p_file := &file // указатель на File
	// и так можно
	read_data(p_file)
}
