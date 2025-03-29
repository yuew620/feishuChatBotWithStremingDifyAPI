package cardcreator

import "context"

// CreateCardFunc defines the function type for creating cards
type CreateCardFunc func(ctx context.Context, content string) (string, error)
