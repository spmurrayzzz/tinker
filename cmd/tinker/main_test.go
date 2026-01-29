package main

import "testing"

func TestComposeQuickstart(t *testing.T) {
	builtIn := "built-in"
	local := "local"
	withLocal := "built-in\n\n## Local workflow instructions\n\nlocal"

	tests := []struct {
		name  string
		mode  string
		local string
		want  string
	}{
		{
			name:  "append no local",
			mode:  "append",
			local: "",
			want:  builtIn,
		},
		{
			name:  "append local",
			mode:  "append",
			local: local,
			want:  withLocal,
		},
		{
			name:  "replace local",
			mode:  "replace",
			local: local,
			want:  local,
		},
		{
			name:  "replace no local",
			mode:  "replace",
			local: "",
			want:  builtIn,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := composeQuickstart(builtIn, test.mode, test.local)
			if got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}
