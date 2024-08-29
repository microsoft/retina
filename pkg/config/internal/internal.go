package internal

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func NewFilteredYAML(source io.Reader, allowedFields []string) (*FilteredYAML, error) {
	f := &FilteredYAML{
		YAML:          source,
		AllowedFields: allowedFields,
		buf:           &bytes.Buffer{},
	}

	if err := f.filter(); err != nil {
		return nil, errors.Wrap(err, "filtering yaml")
	}

	return f, nil
}

// FilteredYAML is a YAML config that is restricted to a specified allowlist of
// fields. Any additional fields found will be removed, such that the resulting
// configuration is the subset of fields found in the allowlist.
type FilteredYAML struct {
	YAML          io.Reader // the input YAML
	AllowedFields []string  // the set of allowed fields in the resulting YAML
	buf           *bytes.Buffer
}

func (f *FilteredYAML) filter() error {
	f.buf = bytes.NewBufferString("")

	decoded := make(map[string]any)
	err := yaml.NewDecoder(f.YAML).Decode(&decoded)
	if err != nil && !errors.Is(err, io.EOF) {
		return errors.Wrap(err, "reading input YAML")
	}

	filtered := make(map[string]any, len(decoded))
	for _, field := range f.AllowedFields {
		if val, ok := decoded[field]; ok {
			filtered[field] = val
		}
	}

	err = yaml.NewEncoder(f.buf).Encode(filtered)
	if err != nil {
		return errors.Wrap(err, "remarshaling filtered yaml")
	}
	return nil
}

// Read extracts the subset of YAML matching AllowedFields and writes it to the
// supplied buffer.
func (f *FilteredYAML) Read(out []byte) (int, error) {
	return f.buf.Read(out)
}
