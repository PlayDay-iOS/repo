package depiction

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildSileoJSON_FullFields(t *testing.T) {
	t.Parallel()
	entry := SileoEntry{
		DisplayName:   "Gram+",
		Section:       "Tweaks",
		Compat:        "iOS 14.0 – 16.x",
		Description:   "Enhances Telegram.",
		Version:       "1.8r-260",
		Architecture:  "iphoneos-arm",
		Maintainer:    "UnlimApps",
		InstalledSize: "4096",
		Depends:       "firmware (>= 14.0)",
	}

	got, err := BuildSileoJSON(entry)
	if err != nil {
		t.Fatalf("BuildSileoJSON failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["class"] != "DepictionStackView" {
		t.Errorf("class = %v, want DepictionStackView", decoded["class"])
	}
	if decoded["tintColor"] != "#2f6690" {
		t.Errorf("tintColor = %v, want #2f6690", decoded["tintColor"])
	}

	views, ok := decoded["views"].([]any)
	if !ok {
		t.Fatalf("views not an array: %T", decoded["views"])
	}
	classes := make([]string, len(views))
	for i, v := range views {
		classes[i] = v.(map[string]any)["class"].(string)
	}
	want := []string{
		"DepictionHeaderView",
		"DepictionSubheaderView", // Section
		"DepictionSubheaderView", // Compat
		"DepictionMarkdownView",
		"DepictionSeparatorView",
		"DepictionTableTextView", // Version
		"DepictionTableTextView", // Architecture
		"DepictionTableTextView", // Maintainer
		"DepictionTableTextView", // Installed-Size
		"DepictionTableTextView", // Depends
	}
	if !reflect.DeepEqual(classes, want) {
		t.Errorf("view classes = %v, want %v", classes, want)
	}
}

func TestBuildSileoJSON_OmitsOptionalFields(t *testing.T) {
	t.Parallel()
	entry := SileoEntry{
		DisplayName:  "Bare",
		Description:  "No extras.",
		Version:      "1.0",
		Architecture: "iphoneos-arm64",
		Maintainer:   "Nobody",
	}

	got, err := BuildSileoJSON(entry)
	if err != nil {
		t.Fatalf("BuildSileoJSON failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	views := decoded["views"].([]any)
	want := []string{
		"DepictionHeaderView",
		"DepictionMarkdownView",
		"DepictionSeparatorView",
		"DepictionTableTextView", // Version
		"DepictionTableTextView", // Architecture
		"DepictionTableTextView", // Maintainer
	}
	if len(views) != len(want) {
		t.Fatalf("view count = %d, want %d. classes: %+v", len(views), len(want), views)
	}
	for i, v := range views {
		cls := v.(map[string]any)["class"].(string)
		if cls != want[i] {
			t.Errorf("view[%d] class = %q, want %q", i, cls, want[i])
		}
	}
}

func TestBuildSileoJSON_DeterministicAcrossRuns(t *testing.T) {
	t.Parallel()
	entry := SileoEntry{
		DisplayName:  "Foo",
		Section:      "Utilities",
		Description:  "x",
		Version:      "1.0",
		Architecture: "iphoneos-arm64",
		Maintainer:   "x",
	}
	a, _ := BuildSileoJSON(entry)
	b, _ := BuildSileoJSON(entry)
	if string(a) != string(b) {
		t.Errorf("output is not deterministic:\n a: %s\n b: %s", a, b)
	}
}
