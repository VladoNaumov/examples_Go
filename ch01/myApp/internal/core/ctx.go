package core

// CtxKey — тип ключей контекста (не строка, чтобы избежать коллизий).
type CtxKey string

// CtxNonce — общий ключ для nonce.
const CtxNonce CtxKey = "nonce"
