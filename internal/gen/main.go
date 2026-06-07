// Command gen reads the vendored uapi OpenAPI spec (openapi.json) plus a
// hand-maintained descriptor overlay (descriptors.go) and emits the uniform
// CRUD/singleton resource and data-source files for the provider, along with a
// registration file. Field names and types (string/int64/bool/list), plus the
// writeOnly/readOnly flags, come straight from the spec; required-ness, nested
// structure (match), kind, and labels come from the overlay. Run via
// `go generate ./...`.
//
// Specials (packages, password, authorized_keys, leases, leases6, token,
// whoami, healthz, diagnostics) and the runtime-bearing interface/wireless data
// sources are hand-written and NOT generated.
package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"sort"
	"strings"
)

type specProp struct {
	Type        any    `json:"type"` // "string" | ["string","null"] | "integer" | "boolean" | "array" | "object"
	WriteOnly   bool   `json:"writeOnly"`
	ReadOnly    bool   `json:"readOnly"`
	Description string `json:"description"`
}

func main() {
	raw, err := os.ReadFile("internal/gen/openapi.json")
	must(err)
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]specProp `json:"properties"`
				Required   []string            `json:"required"`
			} `json:"schemas"`
		} `json:"components"`
	}
	must(json.Unmarshal(raw, &doc))

	var resCtors, dsCtors []string
	for _, d := range descriptors {
		sch, ok := doc.Components.Schemas[d.Schema]
		if !ok {
			fail("schema %q not in spec", d.Schema)
		}
		r := buildResource(d, sch.Properties, sch.Required)
		writeGo(fmt.Sprintf("internal/provider/%s_resource.go", d.Type), renderResource(r))
		resCtors = append(resCtors, "New"+r.Pascal+"Resource")
		writeExample("resources/uapi_"+d.Type, r)
		if d.GenDataSource {
			writeGo(fmt.Sprintf("internal/provider/%s_data_source.go", d.Type), renderDataSource(r))
			dsCtors = append(dsCtors, "New"+r.Pascal+"DataSource")
			writeExample("data-sources/uapi_"+d.Type, r)
		}
	}
	writeGo("internal/provider/zz_generated_registration.go", renderRegistration(resCtors, dsCtors))
	fmt.Printf("generated %d resources, %d data sources\n", len(resCtors), len(dsCtors))
}

// field is a fully-resolved attribute ready to template.
type field struct {
	Name   string // tfsdk + wire name
	GoName string
	GoType string // "types.String" | "types.Int64" | "types.Bool" | "types.List"
	Kind   string // "required" | "optcomp" | "writeonly" | "computedbool" | "computedstring"
	Desc   string
}

type nested struct {
	Name   string
	GoType string
	Fields []field
}

type resModel struct {
	Type       string
	Pascal     string
	Camel      string
	Collection string
	Kind       string // "collection" | "singleton"
	Label      string
	Fields     []field
	Nested     *nested
	GenDS      bool
	Runtime    string
}

func (r resModel) hasCreateOnly() bool {
	for _, f := range r.Fields {
		if f.Kind == "createonly" {
			return true
		}
	}
	return false
}

// dsFields drops create-only fields: they are caller-supplied write inputs the
// API never returns, so they would only ever be null on a (read-only) data
// source. Used by the runtime data source, which has a dedicated DS model.
func (r resModel) dsFields() []field {
	out := make([]field, 0, len(r.Fields))
	for _, f := range r.Fields {
		if f.Kind != "createonly" {
			out = append(out, f)
		}
	}
	return out
}

func buildResource(d descriptor, props map[string]specProp, required []string) resModel {
	r := resModel{
		Type: d.Type, Pascal: pascal(d.Type), Camel: camel(d.Type),
		Collection: d.Collection, Kind: d.Kind, Label: d.Label,
		GenDS: d.GenDataSource, Runtime: d.Runtime,
	}
	// required-ness comes from the spec's top-level `required` array (the
	// unconditional set). `match` is the nested block, handled via d.Nested.
	req := map[string]bool{}
	for _, x := range required {
		if x != "match" {
			req[x] = true
		}
	}
	createOnly := map[string]bool{}
	for _, x := range d.CreateOnly {
		createOnly[x] = true
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if n == "id" || n == "managed" || n == "runtime" {
			continue
		}
		p := props[n]
		if typeOf(p) == "object" {
			continue // nested (match) comes from the descriptor, not the spec
		}
		f := field{Name: n, GoName: pascal(n), Desc: d.Desc(n)}
		// createonly fields (e.g. an interface `name`): caller-supplied at create,
		// immutable, never returned, rejected on PUT/PATCH. Use the spec's own
		// description since these are too special for the commonDesc table.
		if createOnly[n] {
			f.GoType, f.Kind = "types.String", "createonly"
			if p.Description != "" {
				f.Desc = p.Description
			}
			r.Fields = append(r.Fields, f)
			continue
		}
		switch {
		case p.ReadOnly:
			if typeOf(p) == "boolean" {
				f.GoType, f.Kind = "types.Bool", "computedbool"
			} else {
				f.GoType, f.Kind = "types.String", "computedstring"
			}
		case p.WriteOnly:
			f.GoType, f.Kind = "types.String", "writeonly"
		default:
			f.GoType = goType(p)
			if req[n] {
				f.Kind = "required"
			} else {
				f.Kind = "optcomp"
			}
		}
		r.Fields = append(r.Fields, f)
	}
	r.Nested = d.Nested
	return r
}

func typeOf(p specProp) string {
	switch t := p.Type.(type) {
	case string:
		return t
	case []any:
		for _, e := range t {
			if s, _ := e.(string); s != "null" {
				return s
			}
		}
	}
	return "string"
}

func goType(p specProp) string {
	switch typeOf(p) {
	case "integer":
		return "types.Int64"
	case "boolean":
		return "types.Bool"
	case "array":
		return "types.List"
	default:
		return "types.String"
	}
}

func pascal(snake string) string {
	parts := strings.Split(snake, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func camel(snake string) string {
	p := pascal(snake)
	return strings.ToLower(p[:1]) + p[1:]
}

// titleFirst upper-cases only the first letter, leaving the rest of a
// multi-word label untouched (e.g. "unbound srv options" -> "Unbound srv
// options"). Used for the schema description; an article like "A" would read
// wrong against the plural/uncountable singleton labels.
func titleFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func writeGo(path, body string) {
	src, err := format.Source([]byte(body))
	if err != nil {
		// Write unformatted for debugging, then fail loudly.
		_ = os.WriteFile(path+".broken", []byte(body), 0o644)
		fail("gofmt %s: %v", path, err)
	}
	must(os.WriteFile(path, src, 0o644))
}

func writeExample(dir string, r resModel) {
	base := "examples/" + dir
	must(os.MkdirAll(base, 0o755))
	if strings.HasPrefix(dir, "resources/") {
		must(os.WriteFile(base+"/resource.tf", []byte(exampleResource(r)), 0o644))
		if r.Kind == "collection" {
			must(os.WriteFile(base+"/import.sh", []byte(exampleImport(r)), 0o644))
		}
	} else {
		must(os.WriteFile(base+"/data-source.tf", []byte(exampleDataSource(r)), 0o644))
	}
}

func must(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "gen: "+f+"\n", a...)
	os.Exit(1)
}
