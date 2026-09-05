package config

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go-config/internal/document"
)

// Format identifies reader input syntax.
type Format uint8

const (
	FormatYAML Format = iota + 1 // YAML with exactly one document.
	FormatJSON                   // JSON with exactly one value.
)

type fileSource struct {
	path     string
	optional bool
}

// File reads a required .yaml, .yml, or .json file on each load.
func File(path string) Source { return fileSource{path: path} }

// OptionalFile ignores only missing files. All other read errors remain fatal.
func OptionalFile(path string) Source { return fileSource{path: path, optional: true} }
func (s fileSource) Name() string     { return "file:" + s.path }
func (s fileSource) Load(ctx context.Context, d *Descriptor) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	var format Format
	switch strings.ToLower(filepath.Ext(s.path)) {
	case ".yaml", ".yml":
		format = FormatYAML
	case ".json":
		format = FormatJSON
	default:
		return Document{}, sourceError(IssueSource, s.Name(), "unsupported file extension", nil)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if s.optional && errors.Is(err, os.ErrNotExist) {
			return Document{}, nil
		}
		return Document{}, sourceError(IssueSource, s.Name(), "cannot read file", err)
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	return parseSource(b, format, s.Name(), s.path, d)
}

type readerSource struct {
	name   string
	data   []byte
	format Format
	err    error
}

// Reader snapshots r at construction, so repeated and concurrent loads use the
// same bytes. Read failures are reported at Load time. A blocking reader cannot
// be interrupted by a later LoadContext cancellation.
func Reader(name string, r io.Reader, format Format) Source {
	s := readerSource{name: name, format: format}
	if r == nil {
		s.err = errors.New("nil reader")
	} else {
		s.data, s.err = io.ReadAll(r)
	}
	return s
}
func (s readerSource) Name() string { return "reader:" + s.name }
func (s readerSource) Load(ctx context.Context, d *Descriptor) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if s.err != nil {
		return Document{}, sourceError(IssueSource, s.Name(), "cannot read source", nil)
	}
	return parseSource(s.data, s.format, s.Name(), s.name, d)
}

func parseSource(data []byte, format Format, source, file string, d *Descriptor) (Document, error) {
	if format != FormatYAML && format != FormatJSON {
		return Document{}, sourceError(IssueSource, source, "unsupported reader format", nil)
	}
	root, err := document.Parse(data, format == FormatJSON)
	if err != nil {
		return Document{}, parseError(err, source, file)
	}
	if root.Fields == nil {
		return Document{}, sourceError(IssueType, source, "configuration root must be an object", nil)
	}
	out := Document{Values: map[string]RawValue{}}
	if err := checkKnown(root, FieldDescriptor{Kind: KindObject, Children: d.Fields}, "", d.UnknownFields == UnknownFieldsIgnore, source, file, false); err != nil {
		return Document{}, err
	}
	var flatten func([]FieldDescriptor, *document.Node)
	flatten = func(fields []FieldDescriptor, node *document.Node) {
		for _, f := range fields {
			n, ok := node.Fields[f.Name]
			if !ok {
				continue
			}
			if f.Kind == KindObject && n.Fields != nil {
				flatten(f.Children, n)
			} else if f.Kind == KindObject {
				var spread func([]FieldDescriptor)
				spread = func(children []FieldDescriptor) {
					for _, c := range children {
						if c.Kind == KindObject {
							spread(c.Children)
						} else {
							// An invalid container is not a value for any child type.
							out.Values[c.Path] = RawValue{Value: struct{}{}, Present: true, Source: source, Location: &Location{file, n.Line, n.Column}}
						}
					}
				}
				spread(f.Children)
			} else {
				out.Values[f.Path] = RawValue{Value: n.Value, Present: true, Source: source, Location: &Location{file, n.Line, n.Column}}
			}
		}
	}
	flatten(d.Fields, root)
	return out, nil
}

func parseError(err error, source, file string) *Error {
	kind := IssueSyntax
	var pe *document.Error
	loc := &Location{File: file, Line: 1, Column: 1}
	if errors.As(err, &pe) {
		kind = IssueKind(pe.Kind)
		loc.Line, loc.Column = pe.Line, pe.Column
	}
	return &Error{Issues: []Issue{{Kind: kind, Source: source, Message: "invalid source " + string(kind), Location: loc}}}
}

func checkKnown(n *document.Node, f FieldDescriptor, path string, ignore bool, source, file string, secret bool) error {
	secret = secret || f.Secret
	switch f.Kind {
	case KindObject:
		for _, key := range n.Keys() {
			var child *FieldDescriptor
			for i := range f.Children {
				if f.Children[i].Name == key {
					child = &f.Children[i]
					break
				}
			}
			cp := key
			if path != "" {
				cp = path + "." + key
			}
			if secret {
				cp = path
			}
			if child == nil {
				if !ignore {
					return &Error{Issues: []Issue{{Kind: IssueUnknownField, Path: cp, Source: source, Message: "unknown configuration field", Location: &Location{file, n.Fields[key].Line, n.Fields[key].Column}}}}
				}
				continue
			}
			if err := checkKnown(n.Fields[key], *child, cp, ignore, source, file, secret); err != nil {
				return err
			}
		}
	case KindList:
		if f.Item != nil {
			for _, item := range n.Items {
				if err := checkKnown(item, *f.Item, path, ignore, source, file, secret); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// EnvOption customizes environment lookup.
type EnvOption func(*envSource)
type envSource struct{ lookup func(string) (string, bool) }

// WithLookupEnv supplies an isolated lookup function, for tests or embedding.
// It must be safe for concurrent calls when its Source is shared.
func WithLookupEnv(lookup func(string) (string, bool)) EnvOption {
	return func(s *envSource) {
		if lookup != nil {
			s.lookup = lookup
		}
	}
}

// Env looks up only descriptor-declared names at load time. Empty values count
// as present. Lists and maps use JSON, and scalars retain their exact text.
func Env(options ...EnvOption) Source {
	s := envSource{lookup: os.LookupEnv}
	for _, o := range options {
		if o != nil {
			o(&s)
		}
	}
	return s
}
func (s envSource) Name() string { return "env" }
func (s envSource) Load(ctx context.Context, d *Descriptor) (Document, error) {
	out := Document{Values: map[string]RawValue{}}
	var visit func([]FieldDescriptor) error
	visit = func(fields []FieldDescriptor) error {
		for _, f := range fields {
			if err := ctx.Err(); err != nil {
				return err
			}
			if f.Kind == KindObject {
				if err := visit(f.Children); err != nil {
					return err
				}
				continue
			}
			if f.EnvDisabled || f.EnvName == "" {
				continue
			}
			if v, ok := s.lookup(f.EnvName); ok {
				out.Values[f.Path] = RawValue{Value: v, Present: true, Text: true, Source: "env:" + f.EnvName}
			}
		}
		return nil
	}
	if err := visit(d.Fields); err != nil {
		return Document{}, err
	}
	return out, nil
}
