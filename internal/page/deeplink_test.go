package page

import (
	"testing"
)

func TestCydiaDeeplink(t *testing.T) {
	got := CydiaDeeplink("https://example.com/repo/stable/")
	want := "cydia://url/https://cydia.saurik.com/api/share#?source=https%3A%2F%2Fexample.com%2Frepo%2Fstable%2F"
	if got != want {
		t.Errorf("CydiaDeeplink() = %q, want %q", got, want)
	}
}

func TestZebraDeeplink(t *testing.T) {
	got := ZebraDeeplink("https://example.com/repo/stable/")
	want := "zbra://sources/add/https:%2F%2Fexample.com%2Frepo%2Fstable%2F"
	if got != want {
		t.Errorf("ZebraDeeplink() = %q, want %q", got, want)
	}
}

func TestSileoDeeplink(t *testing.T) {
	got := SileoDeeplink("https://example.com/repo/stable/")
	want := "sileo://source/https:%2F%2Fexample.com%2Frepo%2Fstable%2F"
	if got != want {
		t.Errorf("SileoDeeplink() = %q, want %q", got, want)
	}
}
