package core

// CtxKey — тип ключей контекста (не строка, чтобы избежать коллизий).
type CtxKey string

// CtxNonce — общий ключ для nonce.
const CtxNonce CtxKey = "nonce"

// "nonce" в meta-тегах CSP (.Nonce — это одноразовый токен (random string),
//который вставляется в HTML-страницу для защиты от XSS-атак).
