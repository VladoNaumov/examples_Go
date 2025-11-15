package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// ---------- MODELS ----------

type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type Item struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
}

type Cart struct {
	Items []Item  `json:"items"`
	Total float64 `json:"total"`
}

// ---------- SERVICE (CartService) ----------

type CartService struct {
	mu    sync.Mutex
	items map[string]Item // key = Product.ID
}

// NewCartService создаёт CartService
func NewCartService() *CartService {
	return &CartService{
		items: make(map[string]Item),
	}
}

// Add добавляет товар или увеличивает количество (количество должно быть >=1)
func (s *CartService) Add(p Product, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be >= 1")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if it, ok := s.items[p.ID]; ok {
		it.Quantity += qty
		s.items[p.ID] = it
	} else {
		s.items[p.ID] = Item{Product: p, Quantity: qty}
	}
	return nil
}

// Update устанавливает количество (если qty == 0 — удаляет)
func (s *CartService) Update(productID string, qty int) error {
	if qty < 0 {
		return errors.New("quantity must be >= 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if qty == 0 {
		delete(s.items, productID)
		return nil
	}
	if it, ok := s.items[productID]; ok {
		it.Quantity = qty
		s.items[productID] = it
		return nil
	}
	return errors.New("product not found in cart")
}

// Remove удаляет товар
func (s *CartService) Remove(productID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, productID)
}

// Clear очищает корзину
func (s *CartService) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]Item)
}

// Items возвращает срез Item
func (s *CartService) Items() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	return out
}

// Total считает итоговую сумму
func (s *CartService) Total() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total float64
	for _, it := range s.items {
		total += float64(it.Quantity) * it.Product.Price
	}
	return total
}

// ToCart возвращает структуру Cart (Items + Total)
func (s *CartService) ToCart() Cart {
	return Cart{
		Items: s.Items(),
		Total: s.Total(),
	}
}

// ---------- HTTP HANDLERS ----------

var cart = NewCartService()

// helper: write JSON response
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// AddRequest : добавить товар в корзину
type AddRequest struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
}

func handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be >= 1"})
		return
	}
	if req.Product.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product id required"})
		return
	}

	if err := cart.Add(req.Product, req.Quantity); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cart.ToCart())
}

// UpdateRequest : обновить количество для productID (путь /cart/{id})
type UpdateRequest struct {
	Quantity int `json:"quantity"`
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	// ожидаем путь /cart/update?id=<productID> и JSON {quantity: N}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query().Get("id")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query param required"})
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Quantity < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be >= 0"})
		return
	}

	if err := cart.Update(q, req.Quantity); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cart.ToCart())
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, cart.ToCart())
}

func handleRemove(w http.ResponseWriter, r *http.Request) {
	// ожидание /cart/remove?id=<productID>
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	cart.Remove(id)
	writeJSON(w, http.StatusOK, cart.ToCart())
}

func handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cart.Clear()
	writeJSON(w, http.StatusOK, cart.ToCart())
}

// ---------- MAIN ----------

func main() {
	http.HandleFunc("/cart/add", handleAdd)
	http.HandleFunc("/cart/update", handleUpdate)
	http.HandleFunc("/cart/get", handleGet)
	http.HandleFunc("/cart/remove", handleRemove)
	http.HandleFunc("/cart/clear", handleClear)

	fmt.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
