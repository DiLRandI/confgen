package infer

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/internal/document"
	"github.com/DiLRandI/confgen/internal/value"
)

// TypeOverride selects a type during inference. Items and Values must be scalar.
// Other schema metadata belongs in the generated full schema.
type TypeOverride struct {
	Type   config.Kind
	Items  *TypeOverride
	Values *TypeOverride
}

// ParseOverrides reads YAML containing a fields mapping of canonical dotted paths
// to type overrides. It does not read files or accept other schema metadata.
func ParseOverrides(name string, data []byte) (map[string]TypeOverride, error) {
	n, err := document.Parse(data, false)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid overrides: %w", name, err)
	}
	if n.Fields == nil || len(n.Fields) != 1 || n.Fields["fields"] == nil || n.Fields["fields"].Fields == nil {
		return nil, fmt.Errorf("%s: overrides require only a fields mapping", name)
	}
	out := map[string]TypeOverride{}
	for _, path := range n.Fields["fields"].OrderedKeys() {
		rule, err := parseOverride(n.Fields["fields"].Fields[path], path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[path] = rule
	}
	if err := validateOverrides(out); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

func parseOverride(n *document.Node, path string) (TypeOverride, error) {
	var out TypeOverride
	if n.Fields == nil {
		return out, fmt.Errorf("override for %q must be a mapping", path)
	}
	for _, key := range n.OrderedKeys() {
		switch key {
		case "type":
			s, ok := n.Fields[key].Value.(string)
			if !ok {
				return out, fmt.Errorf("override for %q requires a string type", path)
			}
			out.Type = config.Kind(s)
		case "items", "values":
			child := n.Fields[key]
			if child.Fields == nil || len(child.Fields) != 1 || child.Fields["type"] == nil {
				return out, fmt.Errorf("override %q supports only %s.type", path, key)
			}
			s, ok := child.Fields["type"].Value.(string)
			if !ok {
				return out, fmt.Errorf("override %q requires a string %s.type", path, key)
			}
			rule := &TypeOverride{Type: config.Kind(s)}
			if key == "items" {
				out.Items = rule
			} else {
				out.Values = rule
			}
		default:
			return out, fmt.Errorf("override for %q contains unsupported metadata", path)
		}
	}
	return out, nil
}

func overridePaths(rules map[string]TypeOverride) []string {
	paths := make([]string, 0, len(rules))
	for path := range rules {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func scalarOverride(kind config.Kind) bool {
	switch kind {
	case config.KindString, config.KindPath, config.KindBool, config.KindInt, config.KindInt64, config.KindUint, config.KindUint64, config.KindFloat64, config.KindDuration:
		return true
	}
	return false
}

func validateOverrides(rules map[string]TypeOverride) error {
	for _, path := range overridePaths(rules) {
		if ok, _ := regexp.MatchString(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`, path); !ok {
			return fmt.Errorf("override path %q must be a canonical dotted field path", path)
		}
		r := rules[path]
		if r.Items != nil && r.Type != config.KindList || r.Values != nil && r.Type != config.KindMap {
			return fmt.Errorf("override for %q has incompatible collection metadata", path)
		}
		switch r.Type {
		case config.KindList:
			if r.Items == nil {
				return fmt.Errorf("field %q requires an item type", path)
			}
		case config.KindMap:
			if r.Values == nil {
				return fmt.Errorf("field %q requires a value type", path)
			}
		case config.KindObject:
		default:
			if !scalarOverride(r.Type) {
				return fmt.Errorf("field %q has an unsupported override type", path)
			}
		}
		for _, child := range []*TypeOverride{r.Items, r.Values} {
			if child != nil && (!scalarOverride(child.Type) || child.Items != nil || child.Values != nil) {
				return fmt.Errorf("override for %q requires scalar collection member types", path)
			}
		}
	}
	return nil
}

func (i inference) override(n *document.Node, path string, defaults bool, r TypeOverride) (config.FieldDescriptor, error) {
	f := config.FieldDescriptor{Path: path, Kind: r.Type}
	if r.Type == config.KindObject {
		if n.Fields == nil {
			return f, i.fail(n, path, "object override requires an object with inferable fields")
		}
		return i.inferField(n, path, defaults)
	}
	if r.Type == config.KindList {
		f.Item = &config.FieldDescriptor{Kind: r.Items.Type, Path: path + "[]"}
	}
	if r.Type == config.KindMap {
		f.MapValue = &config.FieldDescriptor{Kind: r.Values.Type, Path: path + "{}"}
	}
	if n.Value == nil && n.Fields == nil && n.Items == nil {
		return f, nil
	}
	conflict := func(shape string) (config.FieldDescriptor, error) {
		return f, i.fail(n, path, fmt.Sprintf("override type %q conflicts with %s value", r.Type, shape))
	}
	if n.Fields != nil && r.Type != config.KindMap {
		return conflict("object")
	}
	if n.Items != nil && r.Type != config.KindList {
		return conflict("list")
	}
	var raw any
	copiable := true
	switch r.Type {
	case config.KindList:
		if n.Items == nil {
			return conflict("scalar")
		}
		items := make([]any, len(n.Items))
		for index, item := range n.Items {
			v, err := i.overrideScalar(item, *f.Item, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return f, err
			}
			if v == nil {
				copiable = false
			}
			items[index] = v
		}
		raw = items
	case config.KindMap:
		if n.Fields == nil {
			return conflict("scalar")
		}
		items := map[string]any{}
		for _, key := range n.OrderedKeys() {
			v, err := i.overrideScalar(n.Fields[key], *f.MapValue, path+"{}")
			if err != nil {
				return f, err
			}
			if v == nil {
				copiable = false
			}
			items[key] = v
		}
		raw = items
	default:
		var err error
		raw, err = i.overrideScalar(n, f, path)
		if err != nil {
			return f, err
		}
	}
	if defaults && copiable && raw != nil {
		f.HasDefault = true
		f.Default = raw
	}
	return f, nil
}

func (i inference) overrideScalar(n *document.Node, f config.FieldDescriptor, path string) (any, error) {
	if n.Fields != nil || n.Items != nil {
		return nil, i.fail(n, path, "value is incompatible with scalar override")
	}
	raw, err := n.Scalar()
	if err != nil {
		return nil, i.fail(n, path, err.Error())
	}
	if raw == nil {
		return nil, nil
	}
	if _, issues := value.Convert(f, raw, false, path); len(issues) > 0 {
		return nil, i.fail(n, path, "value is incompatible with override type "+string(f.Kind))
	}
	return raw, nil
}
