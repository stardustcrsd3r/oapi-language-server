package spec

import "testing"

func TestIsOpenAPI(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"openapi", "openapi: 3.0.3\n", true},
		{"swagger", "swagger: \"2.0\"\n", true},
		{"indented later", "# c\nopenapi: 3.1.0\n", true},
		{"plain yaml", "name: foo\nvalues: [1,2]\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := IsOpenAPI([]byte(c.src), 50); got != c.want {
			t.Errorf("%s: IsOpenAPI = %v, want %v", c.name, got, c.want)
		}
	}
}
