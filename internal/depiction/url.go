// Package depiction generates per-package Cydia HTML and Sileo native JSON
// depictions from .deb control data and injects the matching URLs into
// Packages-stanza output.
package depiction

import (
	"net/url"
	"strings"
)

// escapeVersion percent-encodes a Debian version string for use as a URL path
// segment. url.PathEscape is insufficient because it leaves ":" unescaped; we
// use url.QueryEscape (which encodes ":") and restore the "+" → "%20"
// substitution that QueryEscape makes for spaces.
func escapeVersion(v string) string {
	return strings.ReplaceAll(url.QueryEscape(v), "+", "%20")
}

// PackageDepictionURL returns the absolute URL for a package version's
// Cydia-visible HTML depiction. repoURL must end with "/" (enforced by
// config.Load).
func PackageDepictionURL(repoURL, pkg, version string) string {
	return repoURL + "depictions/" + pkg + "/" + escapeVersion(version) + "/depiction.html"
}

// PackageSileoURL returns the absolute URL for a package version's Sileo
// native depiction JSON. repoURL must end with "/".
func PackageSileoURL(repoURL, pkg, version string) string {
	return repoURL + "depictions/" + pkg + "/" + escapeVersion(version) + "/sileo.json"
}
