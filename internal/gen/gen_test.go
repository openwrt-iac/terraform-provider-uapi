package main

import (
	"flag"
	"go/format"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files in testdata/")

// fixtureProps exercises every field-classification branch the generator has:
// required string, optional+computed string/int/bool/list, write-only,
// read-only bool/string, and create-only. buildResource sorts field names, so
// output is stable.
func fixtureProps() map[string]specProp {
	return map[string]specProp{
		"name":       {Type: "string"},
		"weight":     {Type: "integer"},
		"flag":       {Type: "boolean"},
		"tags":       {Type: "array"},
		"secret":     {Type: "string", WriteOnly: true},
		"has_secret": {Type: "boolean", ReadOnly: true},
		"note":       {Type: "string", ReadOnly: true},
		"wgname":     {Type: "string", Description: "Create-only kernel name."},
	}
}

func fixtureNested() *nested {
	return &nested{Name: "match", GoType: "labThingMatch", Fields: []field{
		{Name: "k", GoName: "K", GoType: "types.String", Kind: "required", Desc: "k."},
		{Name: "vs", GoName: "Vs", GoType: "types.List", Kind: "optcomp", Desc: "vs."},
	}}
}

func goldenCases() map[string]string {
	collection := descriptor{
		Type: "lab_thing", Schema: "LabThings", Collection: "lab/things",
		Kind: "collection", Label: "lab thing", GenDataSource: true, Nested: fixtureNested(),
		CreateOnly: []string{"wgname"},
	}
	singleton := descriptor{
		Type: "lab_single", Schema: "LabSingle", Collection: "lab/single",
		Kind: "singleton", Label: "lab single", GenDataSource: true,
	}
	cr := buildResource(collection, fixtureProps(), []string{"name"})
	sr := buildResource(singleton, fixtureProps(), nil)
	return map[string]string{
		"lab_thing_resource.go.golden":    renderResource(cr),
		"lab_thing_data_source.go.golden": renderDataSource(cr),
		"lab_single_resource.go.golden":   renderResource(sr),
	}
}

func TestGoldenRender(t *testing.T) {
	for name, got := range goldenCases() {
		// The emitted source must be valid, gofmt-stable Go; compare the formatted form.
		formatted, err := format.Source([]byte(got))
		if err != nil {
			t.Fatalf("%s: emitted invalid Go: %v", name, err)
		}
		path := filepath.Join("testdata", name)
		if *update {
			if err := os.WriteFile(path, formatted, 0o644); err != nil {
				t.Fatalf("write golden %s: %v", name, err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s (run `go test ./internal/gen -update`): %v", name, err)
		}
		if string(formatted) != string(want) {
			t.Errorf("%s drifted from golden; run `go test ./internal/gen -update` and review the diff", name)
		}
	}
}
