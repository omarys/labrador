package htmlmd_test

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/omarys/labrador/internal/htmlmd"
)

// fragment parses html and converts its body content to a Markdown fragment.
func fragment(t *testing.T, html string) string {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parsing %q: %v", html, err)
	}
	return htmlmd.Fragment(doc.Find("body"))
}

func TestFragment(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "paragraphs joined by one blank line",
			html: `<p>First.</p><p>Second.</p><p>Third.</p>`,
			want: "First.\n\nSecond.\n\nThird.",
		},
		{
			name: "inline emphasis and strong",
			html: `<p>“<em>Fang Yuan</em>, hand it over,” the <strong>elder</strong> said.</p>`,
			want: `“*Fang Yuan*, hand it over,” the **elder** said.`,
		},
		{
			name: "emphasis directly against prose",
			html: `<p>a<em>b</em>c</p>`,
			want: "a*b*c",
		},
		{
			name: "whitespace inside emphasis moves outside the markers",
			html: `<p>x<em> word </em>y</p>`,
			want: "x *word* y",
		},
		{
			name: "nested emphasis",
			html: `<p><strong>a <em>b</em></strong></p>`,
			want: "**a *b***",
		},
		{
			name: "link becomes inline reference",
			html: `<p>See <a href="https://example.com/x">the wiki</a> for more.</p>`,
			want: "See [the wiki](https://example.com/x) for more.",
		},
		{
			name: "in-page anchor keeps text and drops the href",
			html: `<p><a href="#top">Back to top</a></p>`,
			want: "Back to top",
		},
		{
			name: "link without text keeps only surrounding prose",
			html: `<p>x <a href="https://example.com"></a> y</p>`,
			want: "x y",
		},
		{
			name: "link href with spaces is angle bracketed",
			html: `<p><a href="https://example.com/a b">doc</a></p>`,
			want: "[doc](<https://example.com/a b>)",
		},
		{
			name: "br becomes a hard line break",
			html: "<p>line one<br>line two</p>",
			want: "line one\\\nline two",
		},
		{
			name: "consecutive br collapse into one break",
			html: "<p>a<br><br><br>b</p>",
			want: "a\\\nb",
		},
		{
			name: "trailing br is dropped",
			html: "<p>a<br></p>",
			want: "a",
		},
		{
			name: "entities are decoded",
			html: `<p>a &amp; b &lt;c&gt; &#8220;quoted&#8221;</p>`,
			want: "a & b <c> “quoted”",
		},
		{
			name: "source newlines and indentation collapse",
			html: "<p>\n   deep   \n\t thought\n</p>",
			want: "deep thought",
		},
		{
			name: "literal asterisks are escaped",
			html: `<p>2 * 3 = 6 and _not emphasis_</p>`,
			want: `2 \* 3 = 6 and \_not emphasis\_`,
		},
		{
			name: "literal brackets and backticks are escaped",
			html: "<p>[Skill acquired] use `go test` now</p>",
			want: "\\[Skill acquired\\] use \\`go test\\` now",
		},
		{
			name: "a line starting with a hash is escaped",
			html: `<p>#1 in the charts</p>`,
			want: `\#1 in the charts`,
		},
		{
			name: "a line starting with a dash is escaped",
			html: `<p>- not a list</p>`,
			want: `\- not a list`,
		},
		{
			name: "an ordered marker at line start is escaped",
			html: `<p>1. not a list</p>`,
			want: `\1. not a list`,
		},
		{
			name: "mid-line dashes and hashes are left alone",
			html: `<p>300 - 400 years, chapter #3</p>`,
			want: "300 - 400 years, chapter #3",
		},
		{
			name: "blockquote",
			html: `<blockquote><p>Translator note.</p></blockquote>`,
			want: "> Translator note.",
		},
		{
			name: "horizontal rule",
			html: `<p>a</p><hr><p>b</p>`,
			want: "a\n\n---\n\nb",
		},
		{
			name: "nested heading becomes a markdown heading",
			html: `<h2>Volume 2</h2><p>a</p>`,
			want: "## Volume 2\n\na",
		},
		{
			name: "unordered list",
			html: `<ul><li>one</li><li>two</li></ul>`,
			want: "- one\n- two",
		},
		{
			name: "ordered list is numbered",
			html: `<ol><li>one</li><li>two</li></ol>`,
			want: "1. one\n2. two",
		},
		{
			name: "preformatted text becomes ordinary prose",
			html: "<pre>a = 1\nb = 2</pre>",
			want: "a = 1 b = 2",
		},
		{
			name: "preformatted indentation is layout, not code",
			html: "<pre>\n    indented one\n    indented two\n</pre>",
			want: "indented one indented two",
		},
		{
			name: "blank lines in preformatted text separate paragraphs",
			html: "<pre>Dear Sir,\n\nI write to you.\nYours.</pre>",
			want: "Dear Sir,\n\nI write to you. Yours.",
		},
		{
			name: "preformatted prose is still escaped",
			html: "<pre>[System] 2 * 3</pre>",
			want: "\\[System\\] 2 \\* 3",
		},
		{
			name: "preformatted text is never fenced",
			html: "<p>before</p><pre>kept</pre><p>after</p>",
			want: "before\n\nkept\n\nafter",
		},
		{
			name: "inline code keeps its text verbatim",
			html: `<p>the <code>[System] notice</code> appeared</p>`,
			want: "the `[System] notice` appeared",
		},
		{
			name: "inline code cannot hold a backtick",
			html: "<p>a <code>tick`in</code> word</p>",
			want: "a tick\\`in word",
		},
		{
			name: "wrapper elements are transparent",
			html: `<div><div><p>nested</p></div></div>`,
			want: "nested",
		},
		{
			name: "page chrome and media are dropped",
			html: `<p>prose</p><button>Next</button><img src="ad.png" alt="ad"><script>var x=1</script><select><option>a</option></select>`,
			want: "prose",
		},
		{
			name: "inline wrappers keep their text",
			html: `<p>a <span class="x">b</span> c</p>`,
			want: "a b c",
		},
		{
			name: "unknown block element still yields its prose",
			html: `<article><p>body</p></article>`,
			want: "body",
		},
		{
			name: "hidden span attribute is dropped",
			html: `<p>“Fang Yuan, hand it over.”<span hidden=""> Read the latest chapters at example dot com.</span></p>`,
			want: `“Fang Yuan, hand it over.”`,
		},
		{
			name: "valueless hidden attribute is dropped",
			html: `<p>prose<span hidden>Read elsewhere.</span></p>`,
			want: "prose",
		},
		{
			name: "hidden attribute with a value is dropped",
			html: `<p>prose<span hidden="hidden">Read elsewhere.</span></p>`,
			want: "prose",
		},
		{
			name: "inline display:none is dropped",
			html: `<p>prose<span style="display:none"> Find the original chapter at example(.)com.</span></p>`,
			want: "prose",
		},
		{
			name: "inline display: none with a space is dropped",
			html: `<p>prose<span style="color: #fff; display: none;">stolen from elsewhere</span></p>`,
			want: "prose",
		},
		{
			name: "inline visibility:hidden is dropped",
			html: `<p>prose<span style="visibility:hidden">notice</span></p>`,
			want: "prose",
		},
		{
			name: "font-size:0 cloaking is dropped",
			html: `<p>prose<span style="font-size:0">notice</span></p>`,
			want: "prose",
		},
		{
			name: "aria-hidden icons are dropped",
			html: `<p>prose <span aria-hidden="true">icon</span>text</p>`,
			want: "prose text",
		},
		{
			name: "screen-reader-only labels are dropped",
			html: `<p>prose<span class="sr-only">Show menu</span> and more</p>`,
			want: "prose and more",
		},
		{
			name: "utility hidden classes are dropped",
			html: `<p>prose<span class="d-none hidden">notice</span></p>`,
			want: "prose",
		},
		{
			name: "a hidden block is dropped entirely",
			html: `<p>prose</p><div hidden><p>Read at example dot com.</p></div><p>more prose</p>`,
			want: "prose\n\nmore prose",
		},
		{
			name: "a paragraph left empty by hidden content disappears",
			html: `<p>prose</p><p><span hidden>notice</span></p>`,
			want: "prose",
		},
		{
			name: "visible spans and classes are kept",
			html: `<p>a <span class="highlight">b</span> c</p>`,
			want: "a b c",
		},
		{
			name: "prose between hidden notices survives",
			html: `<p>start<span hidden>notice one</span> middle <span style="display:none">notice two</span>end</p>`,
			want: "start middle end",
		},
		{
			name: "empty and whitespace-only fragments",
			html: "<p>   </p>\n<p></p>\n<div> </div>",
			want: "",
		},
		{
			name: "text without any block element becomes one paragraph",
			html: `<section>bare prose</section>`,
			want: "bare prose",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fragment(t, tc.html); got != tc.want {
				t.Errorf("htmlmd.Fragment(%q)\n got: %q\nwant: %q", tc.html, got, tc.want)
			}
		})
	}
}

// TestFragmentRealChapterShape covers the shape novelping's div#chr-content
// actually has: plain <p> per paragraph, curvy quotes, leading spaces, ellipsis
// lines, and no inline markup at all.
func TestFragmentRealChapterShape(t *testing.T) {
	html := `<div id="chr-content" class="chr-c">
		<p> “Fang Yuan, quietly hand over the Spring Autumn Cicada and I’ll give you a quick death!”
</p><p> “Old bastard Fang, stop attempting to resist anymore.”
</p><p> ......
</p><p> In short, it is the ability to be reborn.</p>
	</div>`

	want := strings.Join([]string{
		"“Fang Yuan, quietly hand over the Spring Autumn Cicada and I’ll give you a quick death!”",
		"“Old bastard Fang, stop attempting to resist anymore.”",
		"......",
		"In short, it is the ability to be reborn.",
	}, "\n\n")

	if got := fragment(t, html); got != want {
		t.Errorf("real chapter shape\n got: %q\nwant: %q", got, want)
	}
}

// TestFragmentDropsNothingVisible guards against the converter silently eating
// prose: every word in the source must survive into the output.
func TestFragmentDropsNothingVisible(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<p>one</p><p>two <em>three</em> four</p><p>five <a href="/x">six</a></p>`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	got := htmlmd.Fragment(doc.Find("body"))
	for _, word := range []string{"one", "two", "three", "four", "five", "six"} {
		if !strings.Contains(got, word) {
			t.Errorf("output %q lost %q", got, word)
		}
	}
}

func TestParagraphCollapsesWhitespace(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<p>  a&nbsp;&nbsp;b
		c  </p>`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if got, want := htmlmd.Paragraph(doc.Find("p")), "a b c"; got != want {
		t.Errorf("Paragraph() = %q, want %q", got, want)
	}
}

func TestInlineKeepsTextOnly(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<p>a <b>b</b> <i>c</i></p>`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if got, want := htmlmd.Inline(doc.Find("p")), "a **b** *c*"; got != want {
		t.Errorf("Inline() = %q, want %q", got, want)
	}
}
