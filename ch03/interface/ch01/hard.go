package main

// Вложенные интерфейсы
import "fmt"

type Reader interface {
	read()
}

type Writer interface {
	write(string)
}

type ReaderWriter interface {
	Reader
	Writer
}

type File struct {
	text string
}

func (f *File) read() {
	fmt.Println(f.text)
}
func (f *File) write(message string) {
	f.text = message
	fmt.Println("Запись в файл строки", message)
}

func main() {

	myFile := &File{}
	myFile.write("Hello METANIT.COM")
	myFile.read()
	myFile.write("Hello World")
	myFile.read()
}
