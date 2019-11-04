package common

import (
	"context"
	"time"
)

const DefaultTimeout = time.Minute

// GetContext returns a context with a default timeout for internal communications. Notice that the context does not
// have any security related information attached to it.
func GetContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}
