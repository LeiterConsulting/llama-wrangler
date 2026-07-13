package main

import "testing"

func TestServiceLikeRuntime(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "one", value: "1", want: true},
		{name: "true", value: "true", want: true},
		{name: "trimmed", value: " TRUE ", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LLAMA_WRANGLER_SERVICE_MODE", test.value)
			if got := serviceLikeRuntime(); got != test.want {
				t.Fatalf("serviceLikeRuntime() = %v, want %v", got, test.want)
			}
		})
	}
}
