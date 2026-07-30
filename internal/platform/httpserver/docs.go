package httpserver

// DataEnvelope documents the {"data": ...} success envelope every endpoint
// wraps its payload in (see OK). Swag's `{object} httpserver.DataEnvelope{data=X}`
// syntax composes this with each endpoint's actual payload type.
type DataEnvelope struct {
	Data any `json:"data"`
}

// ErrorEnvelope documents the {"error": "..."} shape written by Error/Fail.
type ErrorEnvelope struct {
	Error string `json:"error"`
}
