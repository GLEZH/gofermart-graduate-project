package loyalty

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type Amount int64

func ParseAmount(value string) (Amount, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "eE+") {
		return 0, ErrInvalidAmount
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidAmount
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	fraction := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, ErrInvalidAmount
		}
		fractionValue := parts[1]
		if len(fractionValue) == 1 {
			fractionValue += "0"
		}
		fraction, err = strconv.ParseInt(fractionValue, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}
	if whole > (int64(^uint64(0)>>1)-fraction)/100 {
		return 0, ErrInvalidAmount
	}
	return Amount(whole*100 + fraction), nil
}

func (a Amount) String() string {
	whole := int64(a) / 100
	fraction := int64(a) % 100
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	if fraction%10 == 0 {
		return fmt.Sprintf("%d.%d", whole, fraction/10)
	}
	return fmt.Sprintf("%d.%02d", whole, fraction)
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	if a == nil || len(data) == 0 || data[0] == '"' || bytes.Equal(data, []byte("null")) {
		return ErrInvalidAmount
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil {
		return ErrInvalidAmount
	}
	value, err := ParseAmount(number.String())
	if err != nil {
		return err
	}
	*a = value
	return nil
}
