package hatena

import (
	"encoding/xml"
	"fmt"
	"time"
)

// XML namespace constants for Hatena Blog AtomPub.
const (
	nsAtom       = "http://www.w3.org/2005/Atom"
	nsApp        = "http://www.w3.org/2007/app"
	nsHatenaBlog = "http://www.hatena.ne.jp/info/xmlns#hatenablog"
)

// Entry represents a single Hatena Blog article.
type Entry struct {
	Title      string
	Content    string
	Date       time.Time
	Updated    time.Time
	Draft      bool
	Categories []string
	URL        string
	EditURL    string
	CustomURL  string
}

// xmlEntry is the internal XML representation of an Atom entry element.
type xmlEntry struct {
	XMLName    xml.Name      `xml:"entry"`
	Xmlns      string        `xml:"xmlns,attr"`
	XmlnsApp   string        `xml:"xmlns:app,attr,omitempty"`
	XmlnsHb    string        `xml:"xmlns:hatenablog,attr,omitempty"`
	Title      string        `xml:"title"`
	Content    xmlContent    `xml:"content"`
	Published  string        `xml:"published,omitempty"`
	Updated    string        `xml:"updated"`
	Links      []xmlLink     `xml:"link"`
	Categories []xmlCategory `xml:"category"`
	Control    *xmlControl   `xml:"http://www.w3.org/2007/app control,omitempty"`
	CustomURL  string        `xml:"http://www.hatena.ne.jp/info/xmlns#hatenablog custom-url,omitempty"`
}

type xmlContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type xmlLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type xmlCategory struct {
	Term string `xml:"term,attr"`
}

type xmlControl struct {
	Draft string `xml:"http://www.w3.org/2007/app draft"`
}

// xmlFeed is the internal XML representation of an Atom feed element.
type xmlFeed struct {
	XMLName xml.Name   `xml:"feed"`
	Links   []xmlLink  `xml:"link"`
	Entries []xmlEntry `xml:"entry"`
}

// parseEntry decodes an XML byte slice into an Entry.
func parseEntry(data []byte) (*Entry, error) {
	var x xmlEntry
	if err := xml.Unmarshal(data, &x); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}
	return entryFromXML(&x), nil
}

// parseFeed decodes an XML byte slice into a list of Entries and the next-page URL.
func parseFeed(data []byte) ([]*Entry, string, error) {
	var f xmlFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, "", fmt.Errorf("unmarshal feed: %w", err)
	}
	var nextURL string
	for _, l := range f.Links {
		if l.Rel == "next" {
			nextURL = l.Href
		}
	}
	entries := make([]*Entry, 0, len(f.Entries))
	for i := range f.Entries {
		entries = append(entries, entryFromXML(&f.Entries[i]))
	}
	return entries, nextURL, nil
}

func entryFromXML(x *xmlEntry) *Entry {
	e := &Entry{
		Title:     x.Title,
		Content:   x.Content.Body,
		CustomURL: x.CustomURL,
	}
	if x.Control != nil && x.Control.Draft == "yes" {
		e.Draft = true
	}
	for _, l := range x.Links {
		switch l.Rel {
		case "alternate":
			e.URL = l.Href
		case "edit":
			e.EditURL = l.Href
		}
	}
	for _, c := range x.Categories {
		e.Categories = append(e.Categories, c.Term)
	}
	if t, err := time.Parse(time.RFC3339, x.Published); err == nil {
		e.Date = t
	}
	if t, err := time.Parse(time.RFC3339, x.Updated); err == nil {
		e.Updated = t
	}
	return e
}

// marshalEntry encodes an Entry into AtomPub XML bytes for create/update requests.
func marshalEntry(e *Entry) ([]byte, error) {
	draft := "no"
	if e.Draft {
		draft = "yes"
	}
	x := xmlEntry{
		Xmlns:     nsAtom,
		XmlnsApp:  nsApp,
		XmlnsHb:   nsHatenaBlog,
		Title:     e.Title,
		Content:   xmlContent{Type: "text/x-markdown", Body: e.Content},
		Control:   &xmlControl{Draft: draft},
		CustomURL: e.CustomURL,
	}
	if !e.Updated.IsZero() {
		x.Updated = e.Updated.Format(time.RFC3339)
	}
	for _, cat := range e.Categories {
		x.Categories = append(x.Categories, xmlCategory{Term: cat})
	}
	out, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal entry: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}
