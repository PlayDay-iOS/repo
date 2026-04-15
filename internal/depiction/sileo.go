package depiction

import "encoding/json"

// SileoEntry is the input bundle for one Sileo native depiction.
type SileoEntry struct {
	DisplayName   string
	Section       string
	Compat        string
	Description   string
	Version       string
	Architecture  string
	Maintainer    string
	InstalledSize string
	Depends       string
}

// sileoStack is the root native-depiction object.
type sileoStack struct {
	Class     string      `json:"class"`
	TintColor string      `json:"tintColor,omitempty"`
	Views     []sileoView `json:"views"`
}

// sileoView is any child node in the stack. Fields are populated per class.
type sileoView struct {
	Class    string `json:"class"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

// BuildSileoJSON assembles the native-depiction JSON for one entry.
// Output is indented for human readability and deterministic because
// struct tags pin field order.
func BuildSileoJSON(e SileoEntry) ([]byte, error) {
	views := []sileoView{
		{Class: "DepictionHeaderView", Title: e.DisplayName},
	}
	if e.Section != "" {
		views = append(views, sileoView{Class: "DepictionSubheaderView", Title: e.Section})
	}
	if e.Compat != "" {
		views = append(views, sileoView{Class: "DepictionSubheaderView", Title: e.Compat})
	}
	views = append(views,
		sileoView{Class: "DepictionMarkdownView", Markdown: e.Description},
		sileoView{Class: "DepictionSeparatorView"},
		sileoView{Class: "DepictionTableTextView", Title: "Version", Text: e.Version},
		sileoView{Class: "DepictionTableTextView", Title: "Architecture", Text: e.Architecture},
		sileoView{Class: "DepictionTableTextView", Title: "Maintainer", Text: e.Maintainer},
	)
	if e.InstalledSize != "" {
		views = append(views, sileoView{Class: "DepictionTableTextView", Title: "Installed-Size", Text: e.InstalledSize})
	}
	if e.Depends != "" {
		views = append(views, sileoView{Class: "DepictionTableTextView", Title: "Depends", Text: e.Depends})
	}
	return json.MarshalIndent(sileoStack{
		Class:     "DepictionStackView",
		TintColor: "#2f6690",
		Views:     views,
	}, "", "  ")
}
