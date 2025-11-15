**Корзину** на Go (prototype).

## План проекта (быстро)

1. `models.go` — определения `Product`, `Item`, `Cart`.
2. `service.go` — `CartService` с методами: `Add`, `Update`, `Remove`, `Clear`, `Total`, `Items`.
3. `handlers.go` — HTTP REST API (JSON): добавить товар, обновить количество, получить корзину, удалить товар, очистить корзину.
4. `main.go` — запуск сервера.
5. `cart_test.go` — unit-тесты для основных методов сервиса.

В коде учтён: JSON-теги, указатели/значения, безопасность для конкурентного доступа (`sync.Mutex`) и простая бизнес-логика (количество >=1, подсчёт total). Замечание: для простоты используем `float64` для цен (в реальном проекте лучше decimal/lib для точных денег).

---

## Примеры запросов (curl)

1. Добавить товар:

```bash
curl -X POST http://localhost:8080/cart/add \
  -H "Content-Type: application/json" \
  -d '{
    "product": {"id":"p1","name":"Shampoo","price":10.5},
    "quantity": 2
  }'
```

2. Получить корзину:

```bash
curl http://localhost:8080/cart/get
```

3. Обновить количество (путь /cart/update?id=p1):

```bash
curl -X POST "http://localhost:8080/cart/update?id=p1" \
  -H "Content-Type: application/json" \
  -d '{"quantity":5}'
```

4. Удалить товар:

```bash
curl -X POST "http://localhost:8080/cart/remove?id=p1"
```

5. Очистить корзину:

```bash
curl -X POST "http://localhost:8080/cart/clear"
```

---

## Unit-tests (файл `cart_test.go`)

```go
// filename: cart_test.go

```



---

## Пояснения / почему так сделано

* `CartService` хранит `map[string]Item` — быстрый доступ по `product.ID`.
* Используем `sync.Mutex` для защиты состояния при одновременных вызовах (в реальном веб-сервере это важно).
* Методы возвращают `error`, где логика может провалиться (qty отрицательное, товар не найден).
* `Cart.ToCart()` формирует удобную JSON-структуру для отдачи в API.
* Для денег лучше использовать типы с фиксированной точностью (в Go есть библиотеки decimal), но для учебных целей `float64` проще.
