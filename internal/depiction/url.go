// Package depiction generates per-package Cydia HTML and Sileo native JSON
// depictions from .deb control data and injects the matching URLs into
// Packages-stanza output.
package depiction

// PackageDepictionURL returns the absolute URL for a .deb's Cydia-visible HTML
// depiction. repoURL must end with "/" (enforced by config.Load). baseName is
// the .deb filename without the ".deb" extension.
func PackageDepictionURL(repoURL, baseName string) string {
	return repoURL + "depictions/" + baseName + "/depiction.html"
}

// PackageSileoURL returns the absolute URL for a .deb's Sileo native depiction
// JSON. repoURL must end with "/".
func PackageSileoURL(repoURL, baseName string) string {
	return repoURL + "depictions/" + baseName + "/sileo.json"
}
