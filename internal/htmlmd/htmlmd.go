// Package htmlmd converts scraped HTML fragments into Markdown.
//
// Text providers scrape prose from a site's DOM and hand it to the archive
// writer, which expects a Markdown fragment: paragraphs separated by one blank
// line, inline emphasis as *…*, **…**, [text](url). This package turns the
// block and inline markup that appears in chapter bodies into that fragment,
// escaping literal prose so it renders unchanged.
//
// The conversion is deliberately conservative. Anything that is not prose —
// page chrome, form controls, embedded media — is dropped rather than
// serialised, and unknown elements are treated as transparent so an
// unexpected wrapper does not swallow a chapter.
package htmlmd

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// hardBreakSentinel marks a <br> while inline text is assembled. Whitespace is
// collapsed before the sentinel becomes a real line break, so the break cannot
// be lost or mistaken for source formatting.
const hardBreakSentinel = "\x00"

var (
	// reSpace collapses runs of whitespace, including the non-breaking spaces
	// that scraped prose is full of.
	reSpace  = regexp.MustCompile(`[ \t\r\n\x{00A0}]+`)
	reBreaks = regexp.MustCompile(`\x00(?: ?\x00)* *`)
)

// inlineSpecials are the characters that change meaning in Markdown and are
// therefore escaped whenever they appear as literal prose. The callers of this
// package deal in prose, not Markdown, so escaping is unconditional.
const inlineSpecials = "\\`*_[]"

// blockMarkers are the characters that start block syntax at the beginning of
// a line. They are escaped only there, so prose such as "300 - 400 years" is
// left alone. Asterisk, underscore, backtick and bracket are absent because
// inlineSpecials already escapes them on every occurrence.
const blockMarkers = "#>-+="

// nonContent elements never carry chapter prose. Dropping them keeps page
// chrome, ad slots and media out of the text.
var nonContent = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true,
	"button": true, "select": true, "option": true, "input": true,
	"form": true, "label": true, "svg": true, "img": true, "picture": true,
	"video": true, "audio": true, "canvas": true, "map": true,
	"nav": true, "aside": true, "footer": true, "head": true, "meta": true,
	"link": true, "template": true, "figure": true,
}

// Fragment renders the block-level content of sel as a Markdown fragment whose
// blocks are separated by one blank line. When sel holds no block elements its
// whole text is rendered as a single paragraph.
func Fragment(sel *goquery.Selection) string {
	var blocks []string
	sel.Contents().Each(func(_ int, c *goquery.Selection) {
		if block := renderBlock(c); block != "" {
			blocks = append(blocks, block)
		}
	})
	if len(blocks) == 0 {
		return Paragraph(sel)
	}
	return strings.Join(blocks, "\n\n")
}

// Paragraph renders one block of prose as a single Markdown paragraph:
// whitespace collapses to single spaces, <br> becomes a hard line break, and
// literal Markdown syntax is escaped.
func Paragraph(sel *goquery.Selection) string {
	text := Inline(sel)
	text = reSpace.ReplaceAllString(text, " ")
	text = strings.TrimRight(text, " \x00")
	text = reBreaks.ReplaceAllString(text, "\\\n")
	return strings.TrimSpace(text)
}

// Inline renders the children of sel — text, emphasis, links, line breaks —
// without adding block structure.
func Inline(sel *goquery.Selection) string {
	w := &inlineWriter{lineStart: true}
	w.children(sel)
	return w.b.String()
}

// renderBlock converts one block-level element, or "" when it holds no prose.
func renderBlock(sel *goquery.Selection) string {
	name := goquery.NodeName(sel)
	if name == "" || name == "#text" || nonContent[name] {
		// Stray text between blocks is formatting noise, not prose.
		return ""
	}

	switch name {
	case "p":
		return Paragraph(sel)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		text := Paragraph(sel)
		if text == "" {
			return ""
		}
		return strings.Repeat("#", int(name[1]-'0')) + " " + text
	case "blockquote":
		return blockquote(Fragment(sel))
	case "hr":
		return "---"
	case "pre":
		code := strings.TrimRight(sel.Text(), "\n")
		if strings.TrimSpace(code) == "" {
			return ""
		}
		return "```\n" + code + "\n```"
	case "ul", "ol":
		return list(sel, name == "ol")
	case "div", "section", "article", "main", "center", "body",
		"table", "tbody", "thead", "tr", "td", "th":
		return Fragment(sel)
	}
	return Paragraph(sel)
}

// blockquote prefixes every line so blank lines inside the quote stay inside it.
func blockquote(fragment string) string {
	if fragment == "" {
		return ""
	}
	lines := strings.Split(fragment, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

func list(sel *goquery.Selection, ordered bool) string {
	var items []string
	sel.ChildrenFiltered("li").Each(func(i int, li *goquery.Selection) {
		text := strings.TrimSpace(Paragraph(li))
		if text == "" {
			return
		}
		marker := "- "
		if ordered {
			marker = strconv.Itoa(len(items)+1) + ". "
		}
		// Continuation lines of one item are indented to stay part of it.
		items = append(items, marker+strings.ReplaceAll(text, "\n", "\n  "))
	})
	return strings.Join(items, "\n")
}

// inlineWriter assembles inline Markdown, tracking whether the next character
// would land at the start of a line.
type inlineWriter struct {
	b         strings.Builder
	lineStart bool
}

func (w *inlineWriter) children(sel *goquery.Selection) {
	sel.Contents().Each(func(_ int, c *goquery.Selection) {
		switch name := goquery.NodeName(c); name {
		case "":
			return
		case "#text":
			w.text(c.Text())
		default:
			w.element(c, name)
		}
	})
}

func (w *inlineWriter) element(sel *goquery.Selection, name string) {
	if nonContent[name] {
		return
	}
	switch name {
	case "br":
		w.hardBreak()
	case "em", "i", "cite", "dfn":
		w.wrap(sel, "*", "*")
	case "strong", "b":
		w.wrap(sel, "**", "**")
	case "code", "kbd", "samp", "tt":
		w.wrap(sel, "`", "`")
	case "del", "s", "strike":
		w.wrap(sel, "~~", "~~")
	case "a":
		w.link(sel)
	case "p", "div":
		// A block nested inside inline content still reads as running text.
		w.text(" ")
		w.children(sel)
		w.text(" ")
	default:
		w.children(sel)
	}
}

// wrap renders sel's children between prefix and suffix, keeping surrounding
// spaces outside the markers so emphasis stays valid Markdown.
func (w *inlineWriter) wrap(sel *goquery.Selection, prefix, suffix string) {
	inner := &inlineWriter{lineStart: w.lineStart}
	inner.children(sel)
	text := inner.b.String()
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if lead := text[:len(text)-len(strings.TrimLeft(text, " \t"))]; lead != "" {
		w.text(lead)
	}
	w.write(prefix + trimmed + suffix)
	if trail := text[len(strings.TrimRight(text, " \t")):]; trail != "" {
		w.text(trail)
	}
}

func (w *inlineWriter) link(sel *goquery.Selection) {
	text := strings.TrimSpace(Inline(sel))
	href, ok := sel.Attr("href")
	href = strings.TrimSpace(href)
	// In-page anchors and script URLs are navigation, not references.
	if text == "" || !ok || href == "" || strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") {
		w.text(text)
		return
	}
	if strings.ContainsAny(href, " ()<>") {
		href = "<" + strings.ReplaceAll(href, ">", "%3E") + ">"
	}
	w.write("[" + text + "](" + href + ")")
}

func (w *inlineWriter) hardBreak() {
	if w.b.Len() == 0 {
		return
	}
	w.b.WriteString(hardBreakSentinel)
	w.lineStart = true
}

func (w *inlineWriter) write(s string) {
	w.b.WriteString(s)
	w.lineStart = false
}

// text appends literal prose, escaping the characters that Markdown would
// otherwise read as syntax.
func (w *inlineWriter) text(s string) {
	if s == "" {
		return
	}
	if w.lineStart {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return
		}
		if strings.IndexByte(blockMarkers, s[0]) >= 0 || isOrderedMarker(s) {
			w.b.WriteByte('\\')
		}
	}
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(inlineSpecials, s[i]) >= 0 {
			w.b.WriteByte('\\')
		}
		w.b.WriteByte(s[i])
	}
	w.lineStart = false
}

// isOrderedMarker reports whether s starts an ordered list ("1. " / "1) ").
func isOrderedMarker(s string) bool {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(s) && (s[i] == '.' || s[i] == ')') && s[i+1] == ' '
}
