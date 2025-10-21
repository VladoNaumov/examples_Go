package core

// context.go

// CtxKey — тип ключей для context.Context (чтобы избежать коллизий строк)
type CtxKey string

const (
	// CtxNonce — ключ для CSP nonce (кладётся в request.Context в middleware)
	CtxNonce CtxKey = "nonce"
)
