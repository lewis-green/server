package messages

import "testing"

func TestIsServiceError(t *testing.T) {
	ptr := func(s string) *string { return &s }

	cases := []struct {
		name string
		err  *string
		want bool
	}{
		{"nil error", nil, false},
		{"empty error", ptr(""), false},
		{
			"no service",
			ptr("Send result: RESULT_ERROR_NO_SERVICE (Failed because service is currently unavailable)"),
			true,
		},
		{"radio off", ptr("Send result: RESULT_ERROR_RADIO_OFF"), true},
		{"generic failure", ptr("Send result: RESULT_ERROR_GENERIC_FAILURE"), false},
		{"invalid recipient", ptr("invalid phone number"), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isServiceError(c.err); got != c.want {
				t.Fatalf("isServiceError() = %v, want %v", got, c.want)
			}
		})
	}
}
