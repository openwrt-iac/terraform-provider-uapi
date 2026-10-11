package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestCronJobWarnings(t *testing.T) {
	cases := []struct {
		name    string
		runtime map[string]any
		want    []string
	}{
		{"healthy", map[string]any{"crond_running": true, "runs_as_written": true}, nil},
		{"procd unreachable", map[string]any{"crond_running": nil, "runs_as_written": true}, nil},
		{"no runtime", nil, nil},
		{"crond stopped", map[string]any{"crond_running": false, "runs_as_written": true}, []string{"crond is not running"}},
		{"misread line", map[string]any{"crond_running": true, "runs_as_written": false}, []string{"Cron job does not run as written"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obj := map[string]any{"schedule": "*/2 * * * *"}
			if c.runtime != nil {
				obj["runtime"] = c.runtime
			}
			var d diag.Diagnostics
			cronJobWarnings(obj, &d)
			if d.HasError() {
				t.Fatalf("warnings only, got errors: %v", d.Errors())
			}
			got := d.Warnings()
			if len(got) != len(c.want) {
				t.Fatalf("want %d warnings, got %d: %v", len(c.want), len(got), got)
			}
			for i, w := range c.want {
				if got[i].Summary() != w {
					t.Errorf("warning %d: want %q, got %q", i, w, got[i].Summary())
				}
			}
		})
	}
}
