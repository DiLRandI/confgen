package config

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"github.com/DiLRandI/confgen/internal/document"
	"github.com/DiLRandI/confgen/internal/value"
)

// Load resolves sources left-to-right, converts only effective values, validates
// requiredness and constraints, and assigns T using descriptor field indices.
// T must be a struct matching the descriptor. It returns nil on every error.
func Load[T any](ctx context.Context, d Descriptor, sources ...Source) (*T, error) {
	if ctx == nil {
		return nil, sourceError(IssueSource, "", "nil context", nil)
	}
	canceled := func() error {
		if err := ctx.Err(); err != nil {
			return sourceError(IssueCanceled, "", "load canceled", err)
		}
		return nil
	}
	if err := canceled(); err != nil {
		return nil, err
	}
	var target T
	if reflect.TypeOf(&target).Elem().Kind() != reflect.Struct {
		return nil, sourceError(IssueType, "", "target must be a struct", nil)
	}
	var leaves []FieldDescriptor
	indices := map[string][]int{}
	state := map[string]RawValue{}
	var walk func([]FieldDescriptor, []int) error
	walk = func(fields []FieldDescriptor, parent []int) error {
		for _, f := range fields {
			index := append(append([]int{}, parent...), f.GoIndex...)
			if f.Kind == KindObject && len(f.Children) > 0 {
				if err := walk(f.Children, index); err != nil {
					return err
				}
			} else {
				if f.Path == "" || len(index) == 0 {
					return sourceError(IssueType, "", "invalid descriptor path or index", nil)
				}
				if _, ok := indices[f.Path]; ok {
					return sourceError(IssueType, "", "duplicate descriptor path", nil)
				}
				indices[f.Path] = index
				leaves = append(leaves, f)
				if f.HasDefault {
					state[f.Path] = RawValue{Value: f.Default, Present: true, Source: "default:" + f.Path}
				}
			}
		}
		return nil
	}
	if err := walk(d.Fields, nil); err != nil {
		return nil, err
	}
	for _, s := range sources {
		if err := canceled(); err != nil {
			return nil, err
		}
		if s == nil {
			return nil, sourceError(IssueSource, "", "nil source", nil)
		}
		doc, err := s.Load(ctx, &d)
		if ce := canceled(); ce != nil {
			return nil, ce
		}
		if err != nil {
			if _, ok := errors.AsType[*Error](err); ok {
				return nil, err
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, sourceError(IssueCanceled, s.Name(), "source canceled", err)
			}
			return nil, sourceError(IssueSource, s.Name(), "source failed", nil)
		}
		keys := make([]string, 0, len(doc.Values))
		for path := range doc.Values {
			keys = append(keys, path)
		}
		sort.Strings(keys)
		for _, path := range keys {
			raw := doc.Values[path]
			if !raw.Present {
				continue
			}
			if _, ok := indices[path]; !ok {
				return nil, &Error{Issues: []Issue{{Kind: IssueUnknownField, Path: path, Source: s.Name(), Message: "unknown canonical field"}}}
			}
			if raw.Source == "" {
				raw.Source = s.Name()
			}
			state[path] = raw
		}
	}
	out := &Error{}
	converted := map[string]any{}
	for _, f := range leaves {
		raw, ok := state[f.Path]
		if !ok {
			if f.Required {
				out.Issues = append(out.Issues, Issue{Kind: IssueRequired, Path: f.Path, Message: "required value is missing"})
			}
			continue
		}
		if raw.Text && (f.Kind == KindList || f.Kind == KindMap) {
			s, ok := raw.Value.(string)
			if !ok {
				out.Issues = append(out.Issues, Issue{Kind: IssueType, Path: f.Path, Source: raw.Source, Message: "expected JSON text"})
				continue
			}
			node, err := document.Parse([]byte(s), true)
			if err != nil {
				pe := parseError(err, raw.Source, "")
				pe.Issues[0].Path = f.Path
				out.Issues = append(out.Issues, pe.Issues...)
				continue
			}
			if err := checkKnown(node, f, f.Path, d.UnknownFields == UnknownFieldsIgnore, raw.Source, "", f.Secret); err != nil {
				out.Issues = append(out.Issues, err.(*Error).Issues...)
				continue
			}
			raw.Value = node.Value
			raw.Text = false
		}
		v, problems := value.Convert(f, raw.Value, raw.Text, f.Path)
		for _, p := range problems {
			message := p.Message
			if f.Secret {
				message += " [REDACTED]"
			}
			out.Issues = append(out.Issues, Issue{Kind: IssueKind(p.Kind), Path: p.Path, Source: raw.Source, Location: raw.Location, Message: message})
		}
		converted[f.Path] = v
	}
	if len(out.Issues) > 0 {
		return nil, out
	}
	if err := canceled(); err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(&target).Elem()
	for _, f := range leaves {
		v, ok := converted[f.Path]
		if !ok {
			continue
		}
		dest, ok := fieldAt(rv, indices[f.Path])
		if !ok || !assign(dest, f, v) {
			return nil, &Error{Issues: []Issue{{Kind: IssueType, Path: f.Path, Message: "target does not match descriptor"}}}
		}
	}
	if err := canceled(); err != nil {
		return nil, err
	}
	return &target, nil
}

func fieldAt(v reflect.Value, index []int) (reflect.Value, bool) {
	for _, i := range index {
		if v.Kind() != reflect.Struct || i < 0 || i >= v.NumField() {
			return reflect.Value{}, false
		}
		v = v.Field(i)
	}
	return v, v.IsValid() && v.CanSet()
}

func assign(dest reflect.Value, f FieldDescriptor, v any) bool {
	if !dest.IsValid() || !dest.CanSet() {
		return false
	}
	switch f.Kind {
	case KindList:
		if dest.Kind() != reflect.Slice || f.Item == nil {
			return false
		}
		a, ok := v.([]any)
		if !ok {
			return false
		}
		out := reflect.MakeSlice(dest.Type(), len(a), len(a))
		for i, item := range a {
			if !assign(out.Index(i), *f.Item, item) {
				return false
			}
		}
		dest.Set(out)
	case KindMap:
		if dest.Kind() != reflect.Map || dest.Type().Key().Kind() != reflect.String || f.MapValue == nil {
			return false
		}
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		out := reflect.MakeMapWithSize(dest.Type(), len(m))
		for k, item := range m {
			x := reflect.New(dest.Type().Elem()).Elem()
			if !assign(x, *f.MapValue, item) {
				return false
			}
			key := reflect.New(dest.Type().Key()).Elem()
			key.SetString(k)
			out.SetMapIndex(key, x)
		}
		dest.Set(out)
	case KindObject:
		m, ok := v.(map[string]any)
		if !ok || dest.Kind() != reflect.Struct {
			return false
		}
		for _, c := range f.Children {
			item, present := m[c.Name]
			if !present {
				continue
			}
			x, ok := fieldAt(dest, c.GoIndex)
			if !ok || !assign(x, c, item) {
				return false
			}
		}
	default:
		x := reflect.ValueOf(v)
		if !x.IsValid() || !x.Type().AssignableTo(dest.Type()) {
			return false
		}
		dest.Set(x)
	}
	return true
}
