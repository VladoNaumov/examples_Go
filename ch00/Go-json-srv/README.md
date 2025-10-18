**Проект JSON API Server ( Go 1.25.1 )**


# 🛡️ JSON API Server - Памятка Разработчика

## 🎯 **Архитектура и назначение**

**Цель**: Безопасный JSON API backend для работы за Nginx reverse proxy.  
**Принципы**: Defense-in-depth, zero-trust, минимум магии, максимум контроля.  
**Nginx роль**: TLS, статика, rate limiting, Host validation.  
**Go роль**: Бизнес-логика, CSRF, CORS, JSON API.

## 🔧 **Константы конфигурации** (верх файла)

```go
const (
    APIAddr = "127.0.0.1:8080"        // Только localhost! Nginx проксирует
    MaxBodyBytes = 10 << 20           // 10MB максимум (JSON + multipart)
    RateLimitMaxRequests = 200        // 200 req/мин per IP
    AllowedOrigins = "https://..."    // ← ИЗМЕНИТЕ НА СВОИ ДОМЕНА!
    CSRFHeaderName = "X-CSRF-Token"   // Обязательный для POST/PUT/DELETE
)
```

**⚠️ Измените `AllowedOrigins` перед продакшеном!**

## 🛡️ **Безопасность - что защищает**

### **1. Rate Limiting (per IP)**
- **In-memory**: `rateLimiter` структура с `sync.RWMutex`
- **Логика**: Скользящее окно (`filterRecent`), очистка старых записей
- **IP извлечение**: `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr`
- **Лимит**: 200 req/мин, дополняет Nginx rate limiting
- **429**: `writeJSON()` с `"rate limited"`

### **2. CSRF защита (API-style)**
- **Триггер**: POST, PUT, PATCH, DELETE (`isStateChanging`)
- **Двойная проверка**:
    1. `Origin` header в вайтлисте (`validateOrigin`)
    2. `X-CSRF-Token` в заголовках (обязательно!)
- **Валидация Origin**: `url.Parse()` + точное сравнение scheme+host
- **Fallback**: Без токена = 403 "CSRF token required"
- **Генерация**: `/api/login` возвращает `csrf_token` в JSON

### **3. CORS (Strict)**
- **Whitelist**: Только `AllowedOrigins` из const
- **Preflight**: OPTIONS → 204 или 403
- **Headers**: `Access-Control-Allow-*` только для валидных origins
- **Credentials**: `true` (cookies работают)
- **Vary**: `Origin` для кэширования
- **Блокировка**: Неизвестный origin → 403 для OPTIONS

### **4. HTTP Security Headers**
- **X-Content-Type-Options**: `nosniff` (MIME sniffing off)
- **X-Frame-Options**: `DENY` (защита от clickjacking)
- **Referrer-Policy**: `strict-origin-when-cross-origin`
- **CSP**: `default-src 'none'; connect-src 'self'` (API-only)
- **Nginx добавляет**: HSTS, дополнительные headers

### **5. Input Validation**
- **Body limit**: `http.MaxBytesReader` (10MB)
- **Header limit**: `MaxHeaderBytes` (1MB)
- **Multipart**: `ParseMultipartForm` с лимитом, `filepath.Base` для filename
- **File extensions**: `.png,.jpg,.jpeg,.gif` whitelist
- **Path traversal**: Блокировка `..`, `/`, `\` в именах файлов

### **6. Session Security**
- **Cookies**: `HttpOnly`, `SameSite=Lax`, `Secure` (если TLS)
- **Token**: 32 байта crypto random (`crypto/rand`)
- **TTL**: 24 часа (`SessionMaxAge`)
- **CSRF токен**: Отдельный, возвращается в `/api/login`

## 🔄 **Middleware Stack** (порядок критичен!)

```go
handler := chain(           // Выполняется снизу вверх
    mux,                    // 9. Роутер
    limitBody(),           // 8. Лимит тела (после CSRF!)
    csrfGuard(),           // 7. CSRF (требует Origin+Token)
    corsStrict(),          // 6. CORS preflight
    secureHeaders(),       // 5. Security headers
    rateLimit(rl),         // 4. Rate limiting
    recoverer(),           // 3. Panic recovery
    requestLogger(),       // 2. Логирование (после status)
)
```

**Почему такой порядок?**
- `limitBody` после CSRF → не тратим ресурсы на атаку
- `recoverer` перед логгером → ловим паники в middleware
- `requestLogger` внешний → захватывает status code

## 📊 **Логирование**

- **Формат**: `METHOD /path 200 HTTP/1.1 ip=1.2.3.4 size=1234 dur=50ms`
- **Без секретов**: Нет body, headers, cookies в логах
- **Status capture**: `responseWriter` wrapper
- **Panic логи**: `log.Printf("panic: %v", rec)`
- **Client IP**: Nginx `X-Real-IP` приоритет

## 🚀 **API Endpoints**

### **`/healthz` GET**
- JSON: `{"status":"ok","version":"1.0"}`
- Без CSRF, rate limit применяется
- Nginx: `access_log off`

### **`/api/login` POST**
```json
// Request
{"username":"user","password":"pass"}

// Response 200
{
  "status":"ok",
  "csrf_token":"a1b2c3d4e5f6...",
  "data":null
}
```
- Устанавливает `session` cookie
- Генерирует CSRF токен
- JSON decode с `json.NewDecoder`

### **`/api/upload` POST**
- **Multipart form**: `file` field
- **CSRF**: `X-CSRF-Token` header ОБЯЗАТЕЛЕН
- **Валидация**: filename, extension, path traversal
- **Response**: `{"status":"uploaded","filename":"img.png","size":12345}`
- **Лимит**: 10MB на файл

## 🌐 **Клиентская интеграция**

### **JavaScript (fetch)**
```javascript
// 1. Login (получаем CSRF)
const login = async (creds) => {
  const res = await fetch('/api/login', {
    method: 'POST',
    credentials: 'include',  // cookies!
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(creds)
  });
  const data = await res.json();
  localStorage.csrfToken = data.csrf_token;  // Сохраняем
};

// 2. Protected request
const upload = async (formData) => {
  const res = await fetch('/api/upload', {
    method: 'POST',
    credentials: 'include',
    headers: { 
      'X-CSRF-Token': localStorage.csrfToken  // ← Обязательно!
    },
    body: formData
  });
};
```

### **cURL пример**
```bash
# Login
curl -c cookies.txt -H "Content-Type: application/json" \
  -d '{"username":"test","password":"test"}' \
  https://api.example.com/api/login

# Upload (с CSRF из login response)
curl -b cookies.txt -H "X-CSRF-Token: a1b2c3..." \
  -F "file=@image.png" \
  https://api.example.com/api/upload
```

## 🛠 **Nginx Integration**

### **Ключевые заголовки от Nginx**
- `X-Real-IP`: Реальный IP клиента
- `X-Forwarded-For`: Цепочка прокси (берем первый)
- `X-Forwarded-Proto`: `https` (для Secure cookies)

### **Nginx config essentials**
```nginx
location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Дополнительный rate limit
    limit_req zone=api burst=20;
}
```

## ⚙️ **Таймауты и лимиты**

- **ReadHeaderTimeout**: 5s (только заголовки)
- **WriteTimeout**: 60s (JSON может быть большим)
- **IdleTimeout**: 5min (keep-alive)
- **Graceful shutdown**: 30s таймаут
- **MaxHeaderBytes**: 1MB (защита от bomb'ов)

## 🔒 **TLS (Nginx responsibility)**

- **Go**: НЕ слушает :443, только localhost:8080
- **Nginx**: TLS 1.2+, HTTP/2, Let's Encrypt
- **HSTS**: В Nginx (`max-age=63072000; preload`)
- **Secure cookies**: Только если `r.TLS != nil` (в проде всегда)

## 🧪 **Тестирование безопасности**

### **CSRF тест**
```bash
# ❌ Должен вернуть 403
curl -X POST https://api.example.com/api/upload \
  -H "Origin: https://evil.com" \
  -F "file=@test.png"

# ✅ С токеном + правильным Origin
curl -X POST https://api.example.com/api/upload \
  -H "Origin: https://example.com" \
  -H "X-CSRF-Token: valid-token" \
  -F "file=@test.png"
```

### **Rate limit тест**
```bash
# 200+ запросов/мин → 429
for i in {1..250}; do curl /healthz & done
```

## 🚨 **Monitoring и алерты**

- **Логи**: Ищите `403 "CSRF"`, `429 "rate limited"`, `panic`
- **Метрики**: Добавьте Prometheus endpoint
- **Health**: `/healthz` для load balancer'ов
- **Graceful shutdown**: SIGTERM → 30s drain

## 📈 **Масштабирование**

- **Rate limiter**: In-memory → Redis для multi-instance
- **Sessions**: Cookie-only → Redis/memcached
- **File uploads**: Temp files → S3/object storage
- **Horizontal scaling**: Stateless + shared session store

## 🔧 **Деплой**

```bash
# 1. Измените константы
const AllowedOrigins = "https://yourdomain.com"

# 2. Build
CGO_ENABLED=0 GOOS=linux go build -o api-server

# 3. Systemd service
[Service]
ExecStart=/path/to/api-server
Restart=always
LimitNOFILE=65536
```

**Готово!** Secure JSON API с полной защитой. 🔒