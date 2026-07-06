package handlers

import "net/http"

// maxRequestBodyBytes caps the size of a decoded JSON request body to guard
// against memory-exhaustion from oversized or slow payloads. Activation
// requests are small (customer details plus a handful of feature configs),
// so 1 MiB is generous.
const maxRequestBodyBytes = 1 << 20

// limitBody wraps the request body with a MaxBytesReader so decoding a body
// larger than maxRequestBodyBytes fails instead of allocating unbounded
// memory.
func limitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
}
