package deb

import (
	"bytes"
	"slices"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/testutil"
)

func TestExtractControlFromReader_MinimalDeb(t *testing.T) {
	t.Parallel()
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "2.1"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@example.com>"},
		{Key: "Description", Value: "A test package"},
	})

	ctrl, err := ExtractControlFromReader(bytes.NewReader(debData), "pkg.deb")
	if err != nil {
		t.Fatalf("ExtractControlFromReader failed: %v", err)
	}

	if ctrl.Get("Package") != "com.test.pkg" {
		t.Errorf("Package = %q", ctrl.Get("Package"))
	}
	if ctrl.Get("Version") != "2.1" {
		t.Errorf("Version = %q", ctrl.Get("Version"))
	}
}

func TestExtractControlFromReader_NotAnAr(t *testing.T) {
	t.Parallel()
	if _, err := ExtractControlFromReader(bytes.NewReader([]byte("not a deb file")), "bad.deb"); err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestNewControlData_AlignsValuesToKeys(t *testing.T) {
	t.Parallel()
	// Mismatched case in keys vs values — values map uses canonical case,
	// keys slice uses Title case as it would appear in the original file.
	c := NewControlData(
		[]string{"Package", "Version"},
		map[string]string{"package": "com.test.pkg", "VERSION": "1.0"},
	)
	if c.Get("Package") != "com.test.pkg" {
		t.Errorf("Get(Package) = %q", c.Get("Package"))
	}
	if c.Get("Version") != "1.0" {
		t.Errorf("Get(Version) = %q", c.Get("Version"))
	}
	if !slices.Equal(c.Order(), []string{"Package", "Version"}) {
		t.Errorf("Order() = %v", c.Order())
	}
}

func TestNewControlData_DropsExtraValues(t *testing.T) {
	t.Parallel()
	// Values not referenced by keys should not appear via Get for those keys
	// the caller did not declare.
	c := NewControlData(
		[]string{"Package"},
		map[string]string{"Package": "p", "Architecture": "arm64"},
	)
	if c.Get("Architecture") != "" {
		t.Errorf("Get(Architecture) = %q, want empty (key not declared)", c.Get("Architecture"))
	}
	if c.Get("Package") != "p" {
		t.Errorf("Get(Package) = %q", c.Get("Package"))
	}
}

func TestControlData_Set_AddsNewKeyToOrder(t *testing.T) {
	t.Parallel()
	c := NewControlData([]string{"Package"}, map[string]string{"Package": "foo"})
	c.Set("Description", "Example package")

	if got := c.Get("Description"); got != "Example package" {
		t.Errorf("Get(Description) = %q", got)
	}
	if got := c.Order(); !slices.Equal(got, []string{"Package", "Description"}) {
		t.Errorf("Order() = %v, want [Package Description]", got)
	}
}

func TestControlData_Set_ReplacesExistingValueInPlace(t *testing.T) {
	t.Parallel()
	c := NewControlData(
		[]string{"Package", "Version"},
		map[string]string{"Package": "foo", "Version": "1.0"},
	)
	c.Set("Package", "bar")

	if got := c.Get("Package"); got != "bar" {
		t.Errorf("Get(Package) = %q, want bar", got)
	}
	if got := c.Order(); !slices.Equal(got, []string{"Package", "Version"}) {
		t.Errorf("Order() = %v, want [Package Version]", got)
	}
}

func TestControlData_Set_CaseInsensitiveMatchPreservesOriginalCase(t *testing.T) {
	t.Parallel()
	c := NewControlData([]string{"Package"}, map[string]string{"Package": "foo"})
	c.Set("package", "bar") // lowercase input on existing key

	if got := c.Get("Package"); got != "bar" {
		t.Errorf("Get(Package) = %q, want bar", got)
	}
	if got := c.Order(); !slices.Equal(got, []string{"Package"}) {
		t.Errorf("Order() = %v, want [Package] (case preserved)", got)
	}
}

func TestControlData_Set_IsIdempotent(t *testing.T) {
	t.Parallel()
	c := NewControlData([]string{"Package"}, map[string]string{"Package": "foo"})
	c.Set("Depiction", "https://example.com/d.html")
	c.Set("Depiction", "https://example.com/d.html")

	if got := c.Order(); !slices.Equal(got, []string{"Package", "Depiction"}) {
		t.Errorf("Order() = %v, want [Package Depiction] (no duplicate)", got)
	}
}
