package internal_test

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/microsoft/retina/pkg/config/internal"
)

func TestFilteredYAML(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		allowed []string
		exp     string
	}{
		{
			"empty",
			"",
			[]string{},
			"{}\n",
		},
		{
			"one field",
			"foo: bar\n",
			[]string{"foo"},
			"foo: bar\n",
		},
		{
			"two fields",
			"foo: bar\nbaz: quux\n",
			[]string{"foo"},
			"foo: bar\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fy, err := internal.NewFilteredYAML(io.NopCloser(strings.NewReader(test.in)), test.allowed)
			if err != nil {
				t.Fatal("unexpected error creating filtered yaml: err:", err)
			}

			got, err := io.ReadAll(fy)
			if err != nil {
				t.Fatal("unexpected error: err:", err)
			}

			if !cmp.Equal(test.exp, string(got)) {
				t.Fatal("yaml differs from expected: diff:", cmp.Diff(test.exp, string(got)))
			}
		})
	}
}
