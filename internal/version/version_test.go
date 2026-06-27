package version

import "testing"

func TestResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		explicit string
		build    string
		want     string
	}{
		{name: "explicit wins", explicit: "v0.2.0", build: "v0.1.0", want: "v0.2.0"},
		{name: "build fallback", explicit: "dev", build: "v0.1.0", want: "v0.1.0"},
		{name: "default dev", explicit: "dev", build: "(devel)", want: "dev"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolve(tc.explicit, tc.build); got != tc.want {
				t.Fatalf("resolve(%q, %q) = %q, want %q", tc.explicit, tc.build, got, tc.want)
			}
		})
	}
}
