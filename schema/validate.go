package schema

import (
	"fmt"
	"go/token"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"go-config/config"
	"go-config/internal/value"
	"gopkg.in/yaml.v3"
)

// Model is a normalized, validated schema. Treat it as immutable. Descriptors
// contain file-representation defaults, never schema YAML or runtime parsers.
type Model struct {
	Package      string
	Name         string
	Descriptor   config.Descriptor
	TypeNames    map[string]string
	Descriptions map[string]string
}

// Compile parses and semantically validates one schema document.
func Compile(filename string, data []byte) (*Model, error) {
	s, err := Parse(filename, data)
	if err != nil {
		return nil, err
	}
	return Validate(s)
}

// Validate checks type compatibility, defaults, constraints, and generated names.
// It returns a normalized model without changing the parsed AST.
func Validate(s *Schema) (*Model, error) {
	if s == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	fail := func(message string) (*Model, error) { return nil, &Diagnostic{Location: s.Location, Message: message} }
	if s.Version != 1 {
		return fail("unsupported schema version; expected 1")
	}
	if !token.IsIdentifier(s.Package) || s.Package == "_" || s.Package == "main" {
		return fail("package must be a non-main Go package identifier")
	}
	if !exported(s.Name) {
		return fail("name must be an exported Go identifier")
	}
	if s.UnknownFields != "error" && s.UnknownFields != "ignore" {
		return fail("unknown_fields must be error or ignore")
	}
	if strings.ContainsAny(s.EnvPrefix, "=\x00\r\n") {
		return fail("invalid env_prefix")
	}
	v := validator{model: &Model{Package: s.Package, Name: s.Name, TypeNames: map[string]string{}, Descriptions: map[string]string{}}, envs: map[string]string{}, types: map[string]bool{"Load": true, "LoadContext": true, "MustLoad": true, "configDescriptor": true, "configRuntime": true, "configContext": true, "configTime": true}}
	if v.types[s.Name] {
		return fail("root name collides with generated helper")
	}
	v.types[s.Name] = true
	v.model.Descriptor = config.Descriptor{RootName: s.Name, EnvPrefix: s.EnvPrefix, UnknownFields: config.UnknownFieldPolicy(s.UnknownFields)}
	fields, err := v.fields(s.Fields, "", false)
	if err != nil {
		return nil, err
	}
	v.model.Descriptor.Fields = fields
	return v.model, nil
}

type validator struct {
	model *Model
	envs  map[string]string
	types map[string]bool
}

func exported(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return token.IsIdentifier(s) && unicode.IsUpper(r)
}

// GoName converts a snake_case field key using a fixed initialism vocabulary.
func GoName(key string) string {
	const initialisms = " API ASCII CPU CSS DNS EOF GUID HTML HTTP HTTPS ID IP JSON QPS RAM RPC SLA SMTP SQL SSH TCP TLS TTL UDP UI UID URI URL UTF8 UUID VM XML XMPP XSRF XSS "
	var b strings.Builder
	for _, part := range strings.Split(key, "_") {
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		if strings.Contains(initialisms, " "+upper+" ") {
			b.WriteString(upper)
		} else {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

func (v *validator) fields(fields []*Field, typePrefix string, inCollection bool) ([]config.FieldDescriptor, error) {
	out := make([]config.FieldDescriptor, 0, len(fields))
	names := map[string]bool{}
	for i, f := range fields {
		if f == nil {
			return nil, fmt.Errorf("nil schema field")
		}
		if ok, _ := regexp.MatchString(`^[a-z][a-z0-9_]*$`, f.Name); !ok {
			return nil, v.fail(f, "invalid field key")
		}
		name := GoName(f.Name)
		if f.Has("go_name") {
			name = f.GoName
		}
		if !exported(name) {
			return nil, v.fail(f, "go_name must be an exported Go identifier")
		}
		if names[name] {
			return nil, v.fail(f, "duplicate generated Go field name")
		}
		names[name] = true
		d, err := v.field(f, typePrefix+name, inCollection)
		if err != nil {
			return nil, err
		}
		d.GoName = name
		d.GoIndex = []int{i}
		out = append(out, d)
	}
	return out, nil
}

func (v *validator) fail(f *Field, message string) error {
	return &Diagnostic{Location: f.Location, Path: f.Path, Message: message}
}

func (v *validator) field(f *Field, typeName string, inCollection bool) (config.FieldDescriptor, error) {
	d := config.FieldDescriptor{Name: f.Name, Path: f.Path, Kind: config.Kind(f.Type), Required: f.Required, Secret: f.Secret, EnvDisabled: inCollection}
	fail := func(msg string) (config.FieldDescriptor, error) { return d, v.fail(f, msg) }
	scalar := false
	numeric := false
	length := false
	switch d.Kind {
	case config.KindString, config.KindPath:
		scalar = true
		length = true
	case config.KindBool:
		scalar = true
	case config.KindInt, config.KindInt64, config.KindUint, config.KindUint64, config.KindFloat64, config.KindDuration:
		scalar = true
		numeric = true
	case config.KindList, config.KindMap:
		length = true
	case config.KindObject:
	default:
		return fail("unsupported field type")
	}
	allowed := map[string]bool{"type": true, "go_name": true, "description": true, "required": d.Kind != config.KindObject, "default": d.Kind != config.KindObject, "secret": d.Kind != config.KindObject, "env": d.Kind != config.KindObject, "enum": scalar, "min": numeric, "max": numeric, "min_length": length, "max_length": length, "pattern": d.Kind == config.KindString || d.Kind == config.KindPath, "fields": d.Kind == config.KindObject, "items": d.Kind == config.KindList, "values": d.Kind == config.KindMap}
	for _, prop := range []string{"required", "default", "secret", "env", "enum", "min", "max", "min_length", "max_length", "pattern", "fields", "items", "values"} {
		if f.Has(prop) && !allowed[prop] {
			return fail(prop + " is incompatible with field type")
		}
	}
	if inCollection && f.Env != nil && f.Env.Tag != "!!bool" {
		return fail("collection members cannot have independent env mappings")
	}
	v.model.Descriptions[f.Path] = f.Description
	switch d.Kind {
	case config.KindObject:
		if !f.Has("fields") {
			return fail("object requires fields")
		}
		tn := typeName + "Config"
		if v.types[tn] {
			return fail("duplicate generated type name")
		}
		v.types[tn] = true
		v.model.TypeNames[f.Path] = tn
		children, err := v.fields(f.Fields, typeName, inCollection)
		if err != nil {
			return d, err
		}
		d.Children = children
	case config.KindList:
		if f.Items == nil {
			return fail("list requires items")
		}
		if f.Items.Type == "list" || f.Items.Type == "map" {
			return fail("nested collections are unsupported")
		}
		item, err := v.field(f.Items, typeName+"Item", true)
		if err != nil {
			return d, err
		}
		d.Item = &item
	case config.KindMap:
		if f.Values == nil {
			return fail("map requires values")
		}
		if f.Values.Type == "object" || f.Values.Type == "list" || f.Values.Type == "map" {
			return fail("map values must be scalar")
		}
		item, err := v.field(f.Values, typeName+"Value", true)
		if err != nil {
			return d, err
		}
		d.MapValue = &item
	}
	if d.Kind != config.KindObject && !inCollection {
		if f.Env != nil && f.Env.Tag == "!!bool" {
			d.EnvDisabled = true
		} else {
			d.EnvName = strings.ToUpper(strings.ReplaceAll(f.Path, ".", "_"))
			if v.model.Descriptor.EnvPrefix != "" {
				d.EnvName = v.model.Descriptor.EnvPrefix + "_" + d.EnvName
			}
			if f.Env != nil {
				d.EnvName = f.Env.Value
			}
			if ok, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, d.EnvName); !ok {
				return fail("invalid environment variable name")
			}
			if _, exists := v.envs[d.EnvName]; exists {
				return fail("duplicate environment variable mapping")
			}
			v.envs[d.EnvName] = f.Path
		}
	}
	decode := func(n *yaml.Node) (any, error) {
		var raw any
		if n == nil {
			return nil, nil
		}
		if err := n.Decode(&raw); err != nil {
			return nil, v.fail(f, "invalid property value")
		}
		return raw, nil
	}
	var err error
	d.Constraints.Min, err = decode(f.Min)
	if err != nil {
		return d, err
	}
	d.Constraints.Max, err = decode(f.Max)
	if err != nil {
		return d, err
	}
	for _, bound := range []struct {
		node *yaml.Node
		raw  any
	}{{f.Min, d.Constraints.Min}, {f.Max, d.Constraints.Max}} {
		if bound.node != nil {
			bf := d
			bf.Constraints = config.Constraints{}
			if _, issues := value.Convert(bf, bound.raw, false, f.Path); len(issues) > 0 {
				return fail("invalid numeric or duration bound")
			}
		}
	}
	if f.Min != nil && f.Max != nil {
		bf := d
		bf.Constraints = config.Constraints{}
		a, _ := value.Convert(bf, d.Constraints.Min, false, f.Path)
		b, _ := value.Convert(bf, d.Constraints.Max, false, f.Path)
		if c, ok := value.Compare(a, b); !ok || c > 0 {
			return fail("min cannot exceed max")
		}
	}
	for _, pair := range []struct {
		node   *yaml.Node
		target **int
	}{{f.MinLength, &d.Constraints.MinLength}, {f.MaxLength, &d.Constraints.MaxLength}} {
		if pair.node == nil {
			continue
		}
		var n int
		if pair.node.Tag != "!!int" || pair.node.Decode(&n) != nil || n < 0 {
			return fail("length bounds must be non-negative integers")
		}
		*pair.target = &n
	}
	if a, b := d.Constraints.MinLength, d.Constraints.MaxLength; a != nil && b != nil && *a > *b {
		return fail("min_length cannot exceed max_length")
	}
	if f.Pattern != nil {
		if f.Pattern.Tag != "!!str" {
			return fail("pattern must be a string")
		}
		if _, err := regexp.Compile(f.Pattern.Value); err != nil {
			return fail("invalid regular expression")
		}
		d.Constraints.Pattern = f.Pattern.Value
	}
	if f.Enum != nil {
		if f.Enum.Kind != yaml.SequenceNode || len(f.Enum.Content) == 0 {
			return fail("enum must be a non-empty sequence")
		}
		var converted []any
		for _, n := range f.Enum.Content {
			raw, e := decode(n)
			if e != nil {
				return d, e
			}
			ef := d
			ef.Constraints = config.Constraints{}
			cv, issues := value.Convert(ef, raw, false, f.Path)
			if len(issues) > 0 {
				return fail("enum value has invalid type")
			}
			for _, prev := range converted {
				if reflect.DeepEqual(prev, cv) {
					return fail("duplicate enum value")
				}
			}
			converted = append(converted, cv)
			d.Constraints.Enum = append(d.Constraints.Enum, raw)
		}
	}
	if f.Default != nil {
		d.Default, err = decode(f.Default)
		if err != nil {
			return d, err
		}
		d.HasDefault = true
		if err := knownDefault(d, d.Default); err != nil {
			return fail("default contains unknown fields")
		}
		if _, issues := value.Convert(d, d.Default, false, f.Path); len(issues) > 0 {
			return fail("default: " + issues[0].Message)
		}
	}
	return d, nil
}

func knownDefault(d config.FieldDescriptor, raw any) error {
	switch d.Kind {
	case config.KindObject:
		if m, ok := raw.(map[string]any); ok {
			for k, x := range m {
				found := false
				for _, c := range d.Children {
					if c.Name == k {
						found = true
						if err := knownDefault(c, x); err != nil {
							return err
						}
					}
				}
				if !found {
					return fmt.Errorf("unknown")
				}
			}
		}
	case config.KindList:
		if a, ok := raw.([]any); ok && d.Item != nil {
			for _, x := range a {
				if err := knownDefault(*d.Item, x); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
