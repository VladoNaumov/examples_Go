package main

import "testing"

func TestAddAndTotal(t *testing.T) {
	s := NewCartService()
	s.Add(Product{ID: "p1", Name: "A", Price: 2.5}, 2)
	s.Add(Product{ID: "p2", Name: "B", Price: 1.0}, 1)
	if got := s.Total(); got != 6.0 {
		t.Fatalf("expected total 6.0, got %v", got)
	}
}

func TestUpdateAndRemove(t *testing.T) {
	s := NewCartService()
	s.Add(Product{ID: "p1", Name: "A", Price: 2.5}, 2)
	if err := s.Update("p1", 5); err != nil {
		t.Fatal(err)
	}
	if got := s.Total(); got != 12.5 {
		t.Fatalf("expected 12.5, got %v", got)
	}
	// remove
	s.Remove("p1")
	if got := s.Total(); got != 0 {
		t.Fatalf("expected 0 after remove, got %v", got)
	}
}

func TestClear(t *testing.T) {
	s := NewCartService()
	s.Add(Product{ID: "p1", Name: "A", Price: 2.5}, 2)
	s.Clear()
	if got := s.Total(); got != 0 {
		t.Fatalf("expected 0 after clear, got %v", got)
	}
}

/*
Запуск тестов:

```bash
go test ./...
```
*/
