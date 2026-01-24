package util

import "context"

// CloneContextValues clones selected keys from the parent context
// into a new background context that is NOT cancellable.
func CloneContextValues(parent context.Context, keys ...interface{}) context.Context {
	// Start from background to avoid cancellation / timeout inheritance
	newCtx := context.Background()

	for _, k := range keys {
		if v := parent.Value(k); v != nil {
			newCtx = context.WithValue(newCtx, k, v)
		}
	}

	return newCtx
}
