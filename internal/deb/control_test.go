package deb

import (
	"bytes"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/testutil"
)

func TestExtractControlFromReader_MinimalDeb(t *testing.T) {
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
	_, err := ExtractControlFromReader(bytes.NewReader([]byte("not a deb file")), "bad.deb")
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}
