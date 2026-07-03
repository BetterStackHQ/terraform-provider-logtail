package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// The dashboards API accepts overlapping chart/section positions, but responds
// by scheduling a debounced server-side layout consolidation that later moves
// items around. Moved items no longer match the coordinates pinned in the
// examples, so the e2e_combined empty-plan check fails whenever a job outlives
// the debounce. This test replays the server's overlap check on the merged
// example configuration: charts are [x, y, w, h] rectangles, sections span the
// full grid width with height 1.
const dashboardGridWidth = 12

type layoutItem struct {
	address    string
	pos        string
	x, y, w, h int
}

func TestExamplesDashboardLayoutHasNoOverlaps(t *testing.T) {
	files, err := exampleTerraformFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no example .tf files found - directory layout changed?")
	}

	parser := hclparse.NewParser()
	itemsByDashboard := map[string][]layoutItem{}
	for _, file := range files {
		f, diags := parser.ParseHCLFile(file)
		if diags.HasErrors() {
			t.Fatalf("parsing %s: %v", file, diags)
		}
		for _, block := range f.Body.(*hclsyntax.Body).Blocks {
			if block.Type != "resource" || len(block.Labels) != 2 {
				continue
			}
			item := layoutItem{
				address: block.Labels[0] + "." + block.Labels[1],
				pos:     fmt.Sprintf("%s:%d", file, block.DefRange().Start.Line),
			}
			attrs := block.Body.Attributes
			switch block.Labels[0] {
			case "logtail_dashboard_chart":
				coords, ok := intAttrs(attrs, "x", "y", "w", "h")
				if !ok {
					continue // no pinned coordinates - the server auto-places without overlap
				}
				item.x, item.y, item.w, item.h = coords[0], coords[1], coords[2], coords[3]
			case "logtail_dashboard_section":
				coords, ok := intAttrs(attrs, "y")
				if !ok {
					continue
				}
				item.x, item.y, item.w, item.h = 0, coords[0], dashboardGridWidth, 1
			default:
				continue
			}
			key := referenceString(attrs["dashboard_id"])
			itemsByDashboard[key] = append(itemsByDashboard[key], item)
		}
	}

	if len(itemsByDashboard) == 0 {
		t.Fatal("no dashboard charts/sections found in examples - parsing broke?")
	}

	for dashboard, items := range itemsByDashboard {
		for i, a := range items {
			for _, b := range items[i+1:] {
				if a.x+a.w > b.x && a.x < b.x+b.w && a.y+a.h > b.y && a.y < b.y+b.h {
					t.Errorf("dashboard %s: %s (%s) overlaps %s (%s) - the server would consolidate the layout and move them, breaking the e2e_combined empty-plan check",
						dashboard, a.address, a.pos, b.address, b.pos)
				}
			}
		}
	}
}

// exampleTerraformFiles mirrors the e2e_combined "Assemble combined examples"
// workflow step: the basic example's scaffolding plus every documented example.
func exampleTerraformFiles() ([]string, error) {
	var files []string
	for _, pattern := range []string{"../../examples/*.tf", "../../examples/resources/*/*.tf", "../../examples/data-sources/*/*.tf"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// intAttrs returns the literal integer values of the named attributes,
// or ok=false if any of them is absent or not a literal number.
func intAttrs(attrs hclsyntax.Attributes, names ...string) ([]int, bool) {
	values := make([]int, 0, len(names))
	for _, name := range names {
		attr, found := attrs[name]
		if !found {
			return nil, false
		}
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !val.Type().IsPrimitiveType() {
			return nil, false
		}
		i, _ := val.AsBigFloat().Int64()
		values = append(values, int(i))
	}
	return values, true
}

// referenceString renders an attribute reference like
// `logtail_dashboard.production.id` back to its traversal string, so charts
// can be grouped by the dashboard resource they attach to.
func referenceString(attr *hclsyntax.Attribute) string {
	if attr == nil {
		return "(none)"
	}
	vars := attr.Expr.Variables()
	if len(vars) == 0 {
		return "(literal)"
	}
	out := ""
	for _, step := range vars[0] {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			out = s.Name
		case hcl.TraverseAttr:
			out += "." + s.Name
		}
	}
	return out
}
