package httpapi

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type userIDKey struct{}

func withUserID(ctx context.Context, userID user.ID) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func userIDFromContext(ctx context.Context) (user.ID, bool) {
	value, ok := ctx.Value(userIDKey{}).(user.ID)
	return value, ok && value > 0
}
