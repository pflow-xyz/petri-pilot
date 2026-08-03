package golang

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// deadGates are feature gates whose backing builders were deleted along with
// the templates they fed. Nothing assigns the Context fields any more, so every
// one of these was permanently false: the `{{if .HasX}}` branches could never be
// taken, and the Context types behind them could never be populated.
//
// This test is the fence. If a gate comes back it must come back with a builder
// that can actually set it — reintroducing the method or the template branch
// alone reinstates dead code that no generated app can ever exercise.
var deadGates = []string{
	"HasSLAs",
	"HasPrediction",
	"HasBlobstore",
	"HasAnyFeatures",
	"HasTimers",
	"HasNotifications",
	"HasRelationships",
	"HasComputed",
	"HasIndexes",
	"HasApprovals",
	"HasTemplates",
	"HasBatch",
	"HasInboundWebhooks",
	"HasDocuments",
	"HasComments",
	"HasTags",
	"HasActivity",
	"HasFavorites",
	"HasExport",
	"HasSoftDelete",
}

// deadFields are the Context fields the dead gates read. They were never
// assigned by any builder.
var deadFields = []string{
	"SLA",
	"Prediction",
	"Blobstore",
	"Timers",
	"Notifications",
	"Relationships",
	"Computed",
	"Indexes",
	"Approvals",
	"Templates",
	"Batch",
	"InboundWebhooks",
	"Documents",
	"Comments",
	"Tags",
	"Activity",
	"Favorites",
	"Export",
	"SoftDelete",
}

func TestNoAlwaysFalseFeatureGates(t *testing.T) {
	ctxType := reflect.TypeOf(&Context{})
	for _, gate := range deadGates {
		if _, ok := ctxType.MethodByName(gate); ok {
			t.Errorf("Context.%s() exists again: it was removed because no builder "+
				"assigns the field it reads, so it can only ever return false", gate)
		}
	}

	structType := ctxType.Elem()
	for _, field := range deadFields {
		if _, ok := structType.FieldByName(field); ok {
			t.Errorf("Context.%s exists again: it was removed because nothing "+
				"assigns it", field)
		}
	}
}

func TestTemplatesReferenceNoDeadGates(t *testing.T) {
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		t.Fatalf("reading templates: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no templates found")
	}

	for _, entry := range entries {
		body, err := fs.ReadFile(templateFS, "templates/"+entry.Name())
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		src := string(body)
		for _, gate := range deadGates {
			if strings.Contains(src, "."+gate) {
				t.Errorf("%s references .%s, a gate that is always false", entry.Name(), gate)
			}
		}
		for _, field := range deadFields {
			// Field access always follows a dot and is followed by a dot,
			// a closing brace, or whitespace — e.g. "{{.SoftDelete.RetentionDays}}"
			// or "{{if .Indexes}}". Method calls on other types (".Type", ".Name")
			// are not in deadFields, so a plain substring scan of "{{...}}" actions
			// would over-match; anchor on the field-access forms instead.
			for _, form := range []string{"." + field + "}}", "." + field + ".", "." + field + " "} {
				if strings.Contains(src, form) {
					t.Errorf("%s references .%s, a Context field that no longer exists", entry.Name(), field)
					break
				}
			}
		}
	}
}
