// Package infer bootstraps configuration contracts from existing YAML or JSON.
// Input values are copied into schema defaults only when Options.CopyDefaults is true.
package infer

import (
	"fmt"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/internal/document"
	"github.com/DiLRandI/confgen/schema"
)

// Options sets the generated package and optional environment prefix.
type Options struct {
	Package   string
	EnvPrefix string
	// CopyDefaults embeds input values in the schema and generated Go.
	// It is false by default.
	CopyDefaults bool
}

// Error describes an ambiguous or unsupported input without echoing its value.
type Error struct {
	Path, Reason string
	Location     schema.Location
}

func (e *Error) Error() string {
	return fmt.Sprintf("cannot infer type for %q: %s; edit the input or define this field in a schema (%s:%d:%d)", e.Path, e.Reason, e.Location.File, e.Location.Line, e.Location.Column)
}

// FromConfig parses a .yaml, .yml, or .json document and returns a validated
// contract. The name is a format/diagnostic label; this function does not read
// or write files. Strings stay strings; integers use int64 or uint64.
func FromConfig(name string, data []byte, options Options) (*schema.Model, error) {
	n, err := document.ParseConfig(name, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return fromNode(name, n, options)
}

func fromNode(name string, n *document.Node, options Options) (*schema.Model, error) {
	i := inference{file: name}
	if n.Fields == nil {
		return nil, i.fail(n, "<root>", "configuration must be an object")
	}
	if options.Package == "" {
		options.Package = "appconfig"
	}
	f, err := i.field(n, "", options.CopyDefaults)
	if err != nil {
		return nil, err
	}
	m := &schema.Model{Package: options.Package, Name: "Config", Descriptor: config.Descriptor{RootName: "Config", EnvPrefix: options.EnvPrefix, Fields: f.Children}}
	b, err := schema.Render(m)
	if err != nil {
		return nil, err
	}
	return schema.Compile(name+" (inferred schema)", b)
}

type inference struct{ file string }

func (i inference) fail(n *document.Node, path, reason string) error {
	return &Error{Path: path, Reason: reason, Location: schema.Location{File: i.file, Line: n.Line, Column: n.Column}}
}

func (i inference) field(n *document.Node, path string, defaults bool) (config.FieldDescriptor, error) {
	f := config.FieldDescriptor{Path: path}
	if n.Fields != nil {
		f.Kind = config.KindObject
		for _, key := range n.OrderedKeys() {
			cp := key
			if path != "" {
				cp = path + "." + key
			}
			c, err := i.field(n.Fields[key], cp, defaults)
			if err != nil {
				return f, err
			}
			c.Name = key
			f.Children = append(f.Children, c)
		}
		return f, nil
	}
	if n.Items != nil {
		f.Kind = config.KindList
		if len(n.Items) == 0 {
			return f, i.fail(n, path, "list is empty and has no item type")
		}
		for index, item := range n.Items {
			c, err := i.field(item, fmt.Sprintf("%s[%d]", path, index+1), false)
			if err != nil {
				return f, err
			}
			if c.Kind == config.KindList {
				return f, i.fail(item, path, "nested lists are unsupported")
			}
			if f.Item == nil {
				f.Item = &c
			} else if !compatible(*f.Item, c) {
				return f, i.fail(item, path, fmt.Sprintf("list items have incompatible types or shapes: item 1 is %s, item %d is %s", f.Item.Kind, index+1, c.Kind))
			}
		}
	} else {
		v, err := n.Scalar()
		if err != nil {
			return f, i.fail(n, path, err.Error())
		}
		switch v.(type) {
		case nil:
			return f, i.fail(n, path, "value is null")
		case string:
			f.Kind = config.KindString
		case bool:
			f.Kind = config.KindBool
		case int64:
			f.Kind = config.KindInt64
		case uint64:
			f.Kind = config.KindUint64
		case float64:
			f.Kind = config.KindFloat64
		default:
			return f, i.fail(n, path, "unsupported scalar representation; quote dates as strings")
		}
	}
	if defaults {
		raw, err := native(n)
		if err != nil {
			return f, i.fail(n, path, err.Error())
		}
		f.HasDefault = true
		f.Default = raw
	}
	return f, nil
}

func native(n *document.Node) (any, error) {
	if n.Fields != nil {
		m := map[string]any{}
		for _, k := range n.OrderedKeys() {
			v, err := native(n.Fields[k])
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		return m, nil
	}
	if n.Items != nil {
		a := make([]any, 0, len(n.Items))
		for _, x := range n.Items {
			v, err := native(x)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		return a, nil
	}
	return n.Scalar()
}

func compatible(a, b config.FieldDescriptor) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == config.KindObject {
		if len(a.Children) != len(b.Children) {
			return false
		}
		for _, x := range a.Children {
			found := false
			for _, y := range b.Children {
				if x.Name == y.Name {
					found = compatible(x, y)
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	if a.Kind == config.KindList {
		return a.Item != nil && b.Item != nil && compatible(*a.Item, *b.Item)
	}
	return true
}
