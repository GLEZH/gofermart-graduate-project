package order

import (
	"errors"
	"strings"
)

var (
	ErrInvalidNumber      = errors.New("invalid order number")
	ErrOwnedByUser        = errors.New("order already uploaded by user")
	ErrOwnedByAnotherUser = errors.New("order uploaded by another user")
	ErrNotFound           = errors.New("order not found")
)

type Number string

func ParseNumber(value string) (Number, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrInvalidNumber
	}

	sum := 0
	double := false
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] < '0' || value[i] > '9' {
			return "", ErrInvalidNumber
		}
		digit := int(value[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	if sum%10 != 0 {
		return "", ErrInvalidNumber
	}

	return Number(value), nil
}

func (n Number) String() string {
	return string(n)
}
