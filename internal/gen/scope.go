package main

import (
	"fmt"
	"sort"
	"strings"
)

// scope is the spec's table of the fields that some values of one
// discriminator do not take, such as the network/interfaces fields that a proto
// does not take. uapi publishes it as `allOf` entries whose `then` makes each
// such field null, and answers 422 to a value for one.
type scope struct {
	Field string
	Known []string // the request enum: the values that uapi models
	Rules []scopeRule
}

// scopeRule: when the discriminator is one of Values, each of Fields is null.
type scopeRule struct {
	Values []string
	Fields []string
}

// applyScope reads the null-only `allOf` entries into r.Scope, and adds to the
// description of each scoped field the values that take it. Other entries, such
// as a field that one proto requires, are left to uapi.
func applyScope(r *resModel, conds []specCond, props map[string]specProp) {
	writable := map[string]bool{}
	for _, f := range r.Fields {
		if f.Kind != "computedbool" && f.Kind != "computedstring" {
			writable[f.Name] = true
		}
	}
	var s scope
	for _, c := range conds {
		if !nullsOnly(r.Type, c) {
			continue
		}
		if len(c.If.Properties) != 1 {
			fail("%s: an allOf entry with `then` properties switches on %d fields; the generator models one discriminator", r.Type, len(c.If.Properties))
		}
		var rule scopeRule
		for name, cond := range c.If.Properties {
			if s.Field != "" && s.Field != name {
				fail("%s: allOf entries switch on both %q and %q; the generator models one discriminator", r.Type, s.Field, name)
			}
			s.Field = name
			rule.Values = cond.Enum
			if cond.Const != nil {
				v, ok := cond.Const.(string)
				if !ok {
					fail("%s: an allOf entry that nulls fields switches on the non-string %q value %v", r.Type, name, cond.Const)
				}
				rule.Values = []string{v}
			}
		}
		for name := range c.Then.Properties {
			if !writable[name] {
				fail("%s: an allOf entry scopes %q, which is not a writable attribute", r.Type, name)
			}
			rule.Fields = append(rule.Fields, name)
		}
		sort.Strings(rule.Fields)
		s.Rules = append(s.Rules, rule)
	}
	if s.Field == "" {
		return
	}
	all := props[s.Field].Enum
	if len(all) == 0 {
		fail("%s: discriminator %q has no enum, so the values that take a scoped field are unknown", r.Type, s.Field)
	}
	s.Known = all
	for i := range r.Fields {
		f := &r.Fields[i]
		excluded := map[string]bool{}
		for _, rule := range s.Rules {
			for _, n := range rule.Fields {
				if n == f.Name {
					for _, v := range rule.Values {
						excluded[v] = true
					}
				}
			}
		}
		if len(excluded) == 0 {
			continue
		}
		var takes []string
		for _, v := range all {
			if !excluded[v] {
				takes = append(takes, "`"+v+"`")
			}
		}
		if len(takes) == 0 {
			fail("%s: no %s takes %q", r.Type, s.Field, f.Name)
		}
		f.Desc += fmt.Sprintf(" Applies only to %s %s.", s.Field, joinAnd(takes))
	}
	r.Scope = &s
}

// nullsOnly reports whether an allOf entry is a scope entry: its `then` sets
// fields to null and does nothing else. An entry that only constrains, such as
// a field that one target requires, is left to uapi. A mix of the two is not
// modelled, so it fails.
func nullsOnly(typ string, c specCond) bool {
	nulls := 0
	for _, p := range c.Then.Properties {
		if typeOf(p) == "null" {
			nulls++
		}
	}
	if nulls > 0 && nulls < len(c.Then.Properties) {
		fail("%s: an allOf entry sets some fields to null and constrains others; the generator models only entries that set fields to null", typ)
	}
	return nulls > 0
}

func joinAnd(xs []string) string {
	if len(xs) == 1 {
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

func goStrings(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = fmt.Sprintf("%q", x)
	}
	return "[]string{" + strings.Join(q, ", ") + "}"
}

// renderScope emits the table and the two methods that apply it: ValidateConfig
// refuses at plan time what uapi refuses at apply, and ModifyPlan nulls what the
// planned value does not take. The logic is in the provider's field_scope.go.
// The table also picks the update method (see crudCollection).
func renderScope(r resModel) string {
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	p("var %sScope = fieldScope{Field: %q, Known: %s, Rules: []scopeRule{", r.Camel, r.Scope.Field, goStrings(r.Scope.Known))
	for _, rule := range r.Scope.Rules {
		p("\t{Values: %s, Fields: %s},", goStrings(rule.Values), goStrings(rule.Fields))
	}
	p("}}\n")
	p("func (r *%sResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {", r.Camel)
	p("\t%sScope.validateConfig(ctx, req, resp)", r.Camel)
	p("}\n")
	p("func (r *%sResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {", r.Camel)
	p("\t%sScope.modifyPlan(ctx, req, resp)", r.Camel)
	p("}\n")
	return b.String()
}
