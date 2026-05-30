package spec

import (
	"reflect"
	"testing"
)

func TestParseRef(t *testing.T) {
	cases := []struct {
		in   string
		want Ref
	}{
		{"#/components/schemas/User", Ref{Value: "#/components/schemas/User", Pointer: "/components/schemas/User"}},
		{"./schemas.yaml#/User", Ref{Value: "./schemas.yaml#/User", File: "./schemas.yaml", Pointer: "/User"}},
		{"./schemas/User.yaml", Ref{Value: "./schemas/User.yaml", File: "./schemas/User.yaml"}},
		{"  #/a/b  ", Ref{Value: "#/a/b", Pointer: "/a/b"}},
		{"#", Ref{Value: "#"}},
	}
	for _, c := range cases {
		got := ParseRef(c.in)
		if got != c.want {
			t.Errorf("ParseRef(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if (got.File == "") != got.IsInternal() {
			t.Errorf("IsInternal mismatch for %q", c.in)
		}
	}
}

func TestPointerSegments(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"/components/schemas/User", []string{"components", "schemas", "User"}},
		{"/a~1b/c~0d", []string{"a/b", "c~d"}},
		{"", nil},
		{"/", nil},
	}
	for _, c := range cases {
		got := PointerSegments(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("PointerSegments(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
