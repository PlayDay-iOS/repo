package page

import (
	"net/url"
)

// CydiaDeeplink returns a cydia:// URL for adding the given source.
// Uses QueryEscape because the source URL is a query parameter value (after #?source=).
func CydiaDeeplink(sourceURL string) string {
	return "cydia://url/https://cydia.saurik.com/api/share#?source=" + url.QueryEscape(sourceURL)
}

// ZebraDeeplink returns a zbra:// URL for adding the given source.
// Uses PathEscape because the source URL is an opaque path segment.
func ZebraDeeplink(sourceURL string) string {
	return "zbra://sources/add/" + url.PathEscape(sourceURL)
}

// SileoDeeplink returns a sileo:// URL for adding the given source.
// Uses PathEscape because the source URL is an opaque path segment.
func SileoDeeplink(sourceURL string) string {
	return "sileo://source/" + url.PathEscape(sourceURL)
}
