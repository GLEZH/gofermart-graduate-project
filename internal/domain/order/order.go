package order

import (
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type Status string

const (
	StatusNew        Status = "NEW"
	StatusProcessing Status = "PROCESSING"
	StatusInvalid    Status = "INVALID"
	StatusProcessed  Status = "PROCESSED"
)

type Order struct {
	Number     Number
	UserID     user.ID
	Status     Status
	Accrual    *loyalty.Amount
	UploadedAt time.Time
	Attempts   int
}

func (s Status) Final() bool {
	return s == StatusInvalid || s == StatusProcessed
}
