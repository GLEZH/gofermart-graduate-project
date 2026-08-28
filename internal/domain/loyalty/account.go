package loyalty

import (
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type Account struct {
	UserID    user.ID
	Current   Amount
	Withdrawn Amount
}

type Withdrawal struct {
	UserID      user.ID
	Order       string
	Amount      Amount
	ProcessedAt time.Time
}
