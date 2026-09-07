package loyalty

import (
	"encoding/json"
	"testing"
)

func TestAmount(t *testing.T) {
	tests := []struct {
		value   string
		want    Amount
		wantStr string
		wantErr bool
	}{
		{value: "0", want: 0, wantStr: "0"},
		{value: "42", want: 4200, wantStr: "42"},
		{value: "500.5", want: 50050, wantStr: "500.5"},
		{value: "1.23", want: 123, wantStr: "1.23"},
		{value: "-1", wantErr: true},
		{value: "1.234", wantErr: true},
		{value: "1e2", wantErr: true},
		{value: ".5", wantErr: true},
		{value: "", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseAmount(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseAmount() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ParseAmount() = %d, want %d", got, test.want)
			}
			if !test.wantErr && got.String() != test.wantStr {
				t.Fatalf("String() = %s, want %s", got.String(), test.wantStr)
			}
		})
	}
}

func TestAmountJSON(t *testing.T) {
	var amount Amount
	if err := json.Unmarshal([]byte("500.5"), &amount); err != nil {
		t.Fatal(err)
	}
	if amount != 50050 {
		t.Fatalf("amount = %d", amount)
	}
	data, err := json.Marshal(amount)
	if err != nil || string(data) != "500.5" {
		t.Fatalf("Marshal() = %s, %v", data, err)
	}
	for _, value := range []string{"null", `"1"`, "-1", "1.001"} {
		if err = json.Unmarshal([]byte(value), &amount); err == nil {
			t.Fatalf("Unmarshal(%s) error = nil", value)
		}
	}
}
