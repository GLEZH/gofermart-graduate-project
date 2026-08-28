package order

import "testing"

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    Number
		wantErr bool
	}{
		{name: "valid", value: "9278923470", want: "9278923470"},
		{name: "valid with spaces", value: " 12345678903\n", want: "12345678903"},
		{name: "zero", value: "0", want: "0"},
		{name: "empty", value: "", wantErr: true},
		{name: "letters", value: "123a", wantErr: true},
		{name: "invalid checksum", value: "12345678901", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseNumber(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseNumber() error = %v", err)
			}
			if got != test.want || got.String() != string(test.want) {
				t.Fatalf("ParseNumber() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStatusFinal(t *testing.T) {
	if StatusNew.Final() || StatusProcessing.Final() || !StatusInvalid.Final() || !StatusProcessed.Final() {
		t.Fatal("unexpected final status result")
	}
}
