package mail

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// allowedTags is a whitelist of safe HTML tags for email rendering.
var allowedTags = map[atom.Atom]bool{
	atom.P:          true,
	atom.Br:         true,
	atom.Hr:         true,
	atom.H1:         true,
	atom.H2:         true,
	atom.H3:         true,
	atom.H4:         true,
	atom.H5:         true,
	atom.H6:         true,
	atom.B:          true,
	atom.I:          true,
	atom.Strong:     true,
	atom.Em:         true,
	atom.U:          true,
	atom.S:          true,
	atom.Strike:     true,
	atom.Del:        true,
	atom.Small:      true,
	atom.Big:        true,
	atom.Sub:        true,
	atom.Sup:        true,
	atom.Blockquote: true,
	atom.Pre:        true,
	atom.Code:       true,
	atom.Ul:         true,
	atom.Ol:         true,
	atom.Li:         true,
	atom.Dl:         true,
	atom.Dt:         true,
	atom.Dd:         true,
	atom.Table:      true,
	atom.Thead:      true,
	atom.Tbody:      true,
	atom.Tfoot:      true,
	atom.Tr:         true,
	atom.Th:         true,
	atom.Td:         true,
	atom.Caption:    true,
	atom.Colgroup:   true,
	atom.Col:        true,
	atom.Div:        true,
	atom.Span:       true,
	atom.A:          true,
	atom.Img:        true,
	atom.Center:     true,
	atom.Font:       true,
	atom.Section:    true,
	atom.Article:    true,
	atom.Main:       true,
	atom.Header:     true,
	atom.Footer:     true,
	atom.Nav:        true,
	atom.Aside:      true,
}

// dropTags are dangerous tags whose entire subtree must be removed.
var dropTags = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Iframe:   true,
	atom.Object:   true,
	atom.Embed:    true,
	atom.Applet:   true,
	atom.Form:     true,
	atom.Input:    true,
	atom.Button:   true,
	atom.Select:   true,
	atom.Textarea: true,
	atom.Meta:     true,
	atom.Link:     true,
	atom.Base:     true,
	atom.Head:     true,
	atom.Frame:    true,
	atom.Frameset: true,
}

// SanitizeHTML parses the input HTML using golang.org/x/net/html and cleans it strictly:
// - Removes script, style, svg, math, form, iframe, and other active/embed elements completely.
// - Strips all event handlers (on* attributes).
// - Strips inline styles to prevent CSS expressions or url() fetch vectors.
// - Sanitizes links (only http/https/mailto, no javascript: or data:).
// - Detects and neutralizes remote images (http, https, //).
// - Safely unwraps unrecognized tags without panicking on node reparenting.
func SanitizeHTML(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}

	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return html.EscapeString(raw), false
	}

	hasRemote := false

	// Clean the DOM tree recursively and safely
	cleanChildren(doc, &hasRemote)

	// Render cleaned DOM to HTML buffer
	var buf bytes.Buffer
	var extractBody func(*html.Node)
	extractBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				_ = html.Render(&buf, c)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractBody(c)
		}
	}

	extractBody(doc)
	if buf.Len() == 0 {
		_ = html.Render(&buf, doc)
	}

	return buf.String(), hasRemote
}

func cleanChildren(parent *html.Node, hasRemote *bool) {
	var next *html.Node
	for c := parent.FirstChild; c != nil; c = next {
		next = c.NextSibling

		if c.Type == html.ElementNode {
			tagLower := strings.ToLower(c.Data)

			// 1. Drop dangerous tags and their whole subtrees
			if dropTags[c.DataAtom] || tagLower == "svg" || tagLower == "math" || tagLower == "script" || tagLower == "style" {
				parent.RemoveChild(c)
				continue
			}

			// 2. If tag is allowed or standard document containers, sanitize and process children
			if allowedTags[c.DataAtom] || c.DataAtom == atom.Html || c.DataAtom == atom.Body || c.DataAtom == atom.Head {
				sanitizeAttributes(c, hasRemote)
				cleanChildren(c, hasRemote)
				continue
			}

			// 3. Unknown or unallowed container element (e.g. custom web component):
			// Clean its children first
			cleanChildren(c, hasRemote)

			// Then safely unwrap c into parent: move each child of c directly before c in parent
			for c.FirstChild != nil {
				ch := c.FirstChild
				c.RemoveChild(ch)
				parent.InsertBefore(ch, c)
			}
			parent.RemoveChild(c)
			continue
		}

		if c.Type == html.CommentNode {
			// Strip HTML comments to prevent conditional IE comments or parser hacks
			parent.RemoveChild(c)
			continue
		}
	}
}

func sanitizeAttributes(n *html.Node, hasRemote *bool) {
	var safeAttrs []html.Attribute
	for _, attr := range n.Attr {
		attrKey := strings.ToLower(attr.Key)

		// Strip all event handlers (onload, onerror, onclick, etc.)
		if strings.HasPrefix(attrKey, "on") {
			continue
		}

		// Strip style attributes entirely to avoid CSS expressions and url() fetches
		if attrKey == "style" {
			continue
		}

		if n.DataAtom == atom.A && attrKey == "href" {
			val := strings.TrimSpace(attr.Val)
			valLower := strings.ToLower(val)
			// Only allow http, https, mailto
			if strings.HasPrefix(valLower, "http://") || strings.HasPrefix(valLower, "https://") || strings.HasPrefix(valLower, "mailto:") {
				safeAttrs = append(safeAttrs, html.Attribute{Key: "href", Val: val})
			}
			continue
		}

		if n.DataAtom == atom.Img && attrKey == "src" {
			val := strings.TrimSpace(attr.Val)
			valLower := strings.ToLower(val)
			// Check for remote URL or protocol-relative URL
			if strings.HasPrefix(valLower, "http://") || strings.HasPrefix(valLower, "https://") || strings.HasPrefix(valLower, "//") {
				*hasRemote = true
				safeAttrs = append(safeAttrs, html.Attribute{Key: "data-blocked-src", Val: val})
				safeAttrs = append(safeAttrs, html.Attribute{Key: "src", Val: "about:blank"})
				safeAttrs = append(safeAttrs, html.Attribute{Key: "title", Val: "Remote image blocked for privacy"})
				continue
			}
			// Allow safe data URIs for inline images
			if strings.HasPrefix(valLower, "data:image/") {
				safeAttrs = append(safeAttrs, attr)
				continue
			}
			continue
		}

		// Whitelist other benign layout and typography attributes
		if attrKey == "title" || attrKey == "alt" || attrKey == "width" || attrKey == "height" ||
			attrKey == "align" || attrKey == "border" || attrKey == "cellpadding" || attrKey == "cellspacing" ||
			attrKey == "colspan" || attrKey == "rowspan" || attrKey == "valign" || attrKey == "class" ||
			attrKey == "color" || attrKey == "face" || attrKey == "size" {
			safeAttrs = append(safeAttrs, attr)
		}
	}

	// For links, enforce target="_blank" and rel="noopener noreferrer"
	if n.DataAtom == atom.A {
		safeAttrs = append(safeAttrs, html.Attribute{Key: "target", Val: "_blank"})
		safeAttrs = append(safeAttrs, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
	}

	n.Attr = safeAttrs
}

// IsSafeURL validates that a URL uses an approved protocol and does not contain script schemes.
func IsSafeURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https" || scheme == "mailto"
}
