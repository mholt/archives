package extensions

import (
	"testing"
)

func TestEndsWith(t *testing.T) {
	for _, tc := range []struct {
		input     string
		extension string
		want      bool
	}{
		{input: "test.tar", extension: "tar", want: true},
		{input: "test.tar", extension: "gz", want: false},
		{input: "test.tar.gz", extension: "tar", want: false},
		{input: "test.tar.gz", extension: "gz", want: true},
		{input: "test.tar.br", extension: "br", want: true},
		{input: "test.tar.br", extension: "bru", want: false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			for _, ext := range []string{tc.extension, "." + tc.extension} {
				got := EndsWith(tc.input, ext)
				if got != tc.want {
					t.Errorf("want: '%v', got: '%v')", tc.want, got)
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	for _, tc := range []struct {
		input     string
		extension string
		want      bool
	}{
		{input: "test.tar", extension: "tar", want: true},
		{input: "test.tar", extension: "gz", want: false},
		{input: "test.tar.gz", extension: "tar", want: true},
		{input: "test.tar.gz", extension: "gz", want: true},
		{input: "test.tar.br", extension: "br", want: true},
		{input: "test.tar.br", extension: "bru", want: false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			for _, ext := range []string{tc.extension, "." + tc.extension} {
				got := Contains(tc.input, ext)
				if got != tc.want {
					t.Errorf("want: '%v', got: '%v')", tc.want, got)
				}
			}
		})
	}
}
