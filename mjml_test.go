package mjml2html

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func document(body string) string { return `<mjml><mj-body>` + body + `</mj-body></mjml>` }
func column(body string) string {
	return `<mj-section><mj-column>` + body + `</mj-column></mj-section>`
}
func requireContains(t *testing.T, s string, items ...string) {
	t.Helper()
	for _, item := range items {
		if !strings.Contains(s, item) {
			t.Errorf("missing %q in output", item)
		}
	}
}

func TestRenderResponsiveEmail(t *testing.T) {
	source := `<mjml lang="hu"><mj-head><mj-title>Próba &amp; cím</mj-title><mj-preview>Előnézet</mj-preview><mj-breakpoint width="520px"/><mj-style>.custom { color:red; }</mj-style><mj-font name="Example" href="https://example.com/font.css"/></mj-head><mj-body background-color="#eee"><mj-section><mj-column><mj-text>Hello <strong>World</strong> &nbsp;!</mj-text><mj-button href="https://example.com/?a=1&amp;b=2">Open</mj-button></mj-column><mj-column><mj-image src="https://example.com/pic.png" alt="Photo"/><mj-divider/><mj-spacer height="30px"/></mj-column></mj-section></mj-body></mjml>`
	out, err := Render(source)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `<!doctype html>`, `lang="hu"`, `<title>Próba &amp; cím</title>`, `Előnézet`, `@media only screen and (min-width:520px)`, `.mj-column-per-50{width:50%!important;max-width:50%;}`, `Hello <strong>World</strong> &nbsp;!`, `href="https://example.com/?a=1&amp;b=2"`, `width:300px;`, `width="250"`, `height:30px;`, `.custom { color:red; }`, `<!--[if mso`)
	if strings.Contains(out, "<mj-") {
		t.Fatal("uncompiled MJML in output")
	}
}

func TestAttributePrecedence(t *testing.T) {
	source := `<mjml><mj-head><mj-attributes><mj-all font-family="Arial" color="black"/><mj-text color="blue"/><mj-class name="first" color="green"/><mj-class name="second" color="orange" font-size="18px"/></mj-attributes></mj-head><mj-body>` + column(`<mj-text mj-class="first second" color="purple">Hi</mj-text>`) + `</mj-body></mjml>`
	out, err := Render(source)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `color:purple;`, `font-size:18px;`, `font-family:Arial;`)
}

func TestRawContent(t *testing.T) {
	raw := `<!-- keep --><p data-x="a > b">One<br>Two &copy; <img src="x"></p>`
	out, err := Render(document(`<mj-raw>` + raw + `</mj-raw>` + column(`<mj-text>`+raw+`</mj-text><mj-table><tr><td>Cell</td></tr></mj-table>`)))
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, raw, `<tr><td>Cell</td></tr>`)
}

func TestTemplateEscapesData(t *testing.T) {
	out, err := RenderTemplate(document(column(`<mj-text>{{.Name}}</mj-text><mj-button href="{{.URL}}">Click</mj-button>`)), map[string]string{"Name": `<script>alert("x")</script>`, "URL": "https://example.com/?a=1&b=2"})
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `&lt;script&gt;`, `href="https://example.com/?a=1&amp;b=2"`)
	if strings.Contains(out, `<script>`) {
		t.Fatal("data was not escaped")
	}
	_, err = RenderTemplate(document(column(`<mj-text>{{.Missing}}</mj-text>`)), map[string]string{})
	if err == nil {
		t.Fatal("missing template key accepted")
	}
}

func TestIncludes(t *testing.T) {
	files := fstest.MapFS{"parts/section.mjml": {Data: []byte(`<mj-section><mj-column><mj-include path="text.mjml"/></mj-column></mj-section>`)}, "parts/text.mjml": {Data: []byte(`<mj-text>Included</mj-text>`)}, "raw.html": {Data: []byte(`<p>Raw</p>`)}, "style.css": {Data: []byte(`.foo{color:red}`)}}
	source := `<mjml><mj-head><mj-include path="style.css" type="css"/></mj-head><mj-body><mj-include path="parts/section.mjml"/><mj-include path="raw.html" type="html"/></mj-body></mjml>`
	out, err := Render(source, Options{IncludeFS: files})
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `Included`, `<p>Raw</p>`, `.foo{color:red}`)
	for _, p := range []string{"../secret", "/etc/passwd", "absent.mjml"} {
		if _, err := Render(document(`<mj-include path="`+p+`"/>`), Options{IncludeFS: files}); err == nil {
			t.Errorf("accepted include %s", p)
		}
	}
	cycle := fstest.MapFS{"cycle.mjml": {Data: []byte(`<mj-include path="cycle.mjml"/>`)}}
	if _, err := Render(document(`<mj-include path="cycle.mjml"/>`), Options{IncludeFS: cycle}); err == nil {
		t.Fatal("accepted include cycle")
	}
}

func TestInvalidInput(t *testing.T) {
	cases := []string{"", `<mjml/>`, `<mjml><mj-body></mjml>`, `<mjml/><mjml/>`, document(`<mj-text>bad parent</mj-text>`), document(column(`<mj-unknown/>`)), document(column(`<mj-hero/>`)), document(`<mj-include path="x"/>`), `<mjml><mj-head><mj-style inline="inline">p{color:red}</mj-style></mj-head><mj-body/></mjml>`, document(column(`<mj-text color=red>bad</mj-text>`)), document(column(`<mj-text color="a" color="b"/>`)), document(column(`<mj-text>missing end`)), `<mjml><mj-body/><mj-body/></mjml>`}
	for i, s := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if out, err := Render(s); err == nil || out != "" {
				t.Fatalf("expected error and no HTML, got %q, %v", out, err)
			}
		})
	}
}

func TestGroupsAndPixelColumns(t *testing.T) {
	out, err := Render(document(`<mj-section padding="0 20px"><mj-group><mj-column width="200px"><mj-text>A</mj-text></mj-column><mj-column><mj-text>B</mj-text></mj-column></mj-group></mj-section>`))
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `.mj-column-px-200{width:200px!important;max-width:200px;}`, `width:35.714285714285715%;`, `width:280px;`)
}

func TestNavigationAndSocial(t *testing.T) {
	out, err := Render(document(column(`<mj-navbar><mj-navbar-link href="https://example.com">Home</mj-navbar-link></mj-navbar><mj-social><mj-social-element src="https://example.com/icon.png" href="https://example.com">Follow</mj-social-element></mj-social>`)))
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, `Home</a>`, `Follow</a>`, `src="https://example.com/icon.png"`)
}

func TestConcurrentRender(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Render(document(column(`<mj-text>Hi</mj-text>`))); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestWelcomeExample(t *testing.T) {
	source, err := os.ReadFile("examples/welcome.mjml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderTemplate(string(source), struct{ Name string }{Name: "Anna & Béla"})
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, out, "Szia, Anna &amp; Béla!", "background-color:#2563eb;")
}

func TestUnsupportedFeatures(t *testing.T) {
	for _, content := range []string{
		`<mj-navbar hamburger="hamburger"/>`,
		`<mj-social><mj-social-element name="facebook">Share</mj-social-element></mj-social>`,
	} {
		if _, err := Render(document(column(content))); err == nil {
			t.Fatal("accepted unsupported feature")
		}
	}
}

func FuzzRender(f *testing.F) {
	for _, seed := range []string{document(column(`<mj-text>Hello</mj-text>`)), `<mjml/>`, `<!--`, `<mjml><mj-body a=" > "/></mjml>`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 100000 {
			t.Skip()
		}
		_, _ = Render(s)
	})
}
