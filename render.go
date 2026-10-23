package mjml2html

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
)

type attributes map[string]string
type renderer struct {
	all                                   attributes
	defaults, classes                     map[string]attributes
	media                                 map[string]string
	styles, fonts                         []string
	title, preview, breakpoint, lang, dir string
}

func newRenderer() *renderer {
	return &renderer{all: attributes{}, defaults: map[string]attributes{}, classes: map[string]attributes{}, media: map[string]string{}, breakpoint: "480px", lang: "und", dir: "auto"}
}

var parents = map[string]string{
	"mj-head":           "mjml",
	"mj-body":           "mjml",
	"mj-title":          "mj-head",
	"mj-preview":        "mj-head",
	"mj-style":          "mj-head",
	"mj-font":           "mj-head",
	"mj-breakpoint":     "mj-head",
	"mj-attributes":     "mj-head",
	"mj-wrapper":        "mj-body",
	"mj-section":        "mj-body mj-wrapper",
	"mj-column":         "mj-section mj-group",
	"mj-group":          "mj-section",
	"mj-text":           "mj-column",
	"mj-image":          "mj-column",
	"mj-button":         "mj-column",
	"mj-divider":        "mj-column",
	"mj-spacer":         "mj-column",
	"mj-table":          "mj-column",
	"mj-raw":            "mj-head mj-body mj-wrapper mj-section mj-column mj-group mj-navbar mj-social",
	"mj-navbar":         "mj-column",
	"mj-navbar-link":    "mj-navbar",
	"mj-social":         "mj-column",
	"mj-social-element": "mj-social",
}

func validate(n *node, parent string) error {
	if n.tag == "mj-attributes" {
		for _, c := range n.children {
			if c.tag != "mj-all" && c.tag != "mj-class" {
				if _, ok := parents[c.tag]; !ok {
					return fmt.Errorf("mjml: unsupported attribute component <%s>", c.tag)
				}
			}
			if len(c.children) > 0 {
				return fmt.Errorf("mjml: attribute definitions cannot contain children")
			}
		}
		return nil
	}
	if n.tag != "mjml" {
		allowed, ok := parents[n.tag]
		if !ok {
			return fmt.Errorf("mjml: unsupported component <%s>", n.tag)
		}
		if !strings.Contains(" "+allowed+" ", " "+parent+" ") {
			return fmt.Errorf("mjml: <%s> is not allowed in <%s>", n.tag, parent)
		}
	}
	for _, c := range n.children {
		if err := validate(c, n.tag); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) render(root *node) (string, error) {
	if err := validate(root, ""); err != nil {
		return "", err
	}
	var body *node
	heads := 0
	for _, n := range root.children {
		if n.tag == "mj-body" {
			if body != nil {
				return "", fmt.Errorf("mjml: duplicate mj-body")
			}
			body = n
		}
		if n.tag == "mj-head" {
			heads++
			if heads > 1 {
				return "", fmt.Errorf("mjml: duplicate mj-head")
			}
			if err := r.head(n); err != nil {
				return "", err
			}
		}
	}
	if body == nil {
		return "", fmt.Errorf("mjml: missing mj-body")
	}
	if root.attrs["lang"] != "" {
		r.lang = root.attrs["lang"]
	}
	if root.attrs["dir"] != "" {
		r.dir = root.attrs["dir"]
	}
	a := r.attrs(body)
	width := length(a["width"], 600)
	if width <= 0 {
		return "", fmt.Errorf("mjml: body width must be positive")
	}
	if err := r.checkFeatures(body); err != nil {
		return "", err
	}
	content := r.children(body, width, false)
	var css strings.Builder
	css.WriteString(baseCSS)
	css.WriteString("@media only screen and (min-width:" + r.breakpoint + "){")
	keys := sortedKeys(r.media)
	for _, k := range keys {
		css.WriteString("." + k + "{width:" + r.media[k] + "!important;max-width:" + r.media[k] + ";}")
	}
	css.WriteString("}")
	css.WriteString("@media only screen and (max-width:" + px(length(r.breakpoint, 480)-1) + "){.mj-full-width-mobile{width:100%!important;max-width:100%!important;}}")
	for _, s := range r.styles {
		css.WriteString(s)
	}
	fonts := strings.Join(r.fonts, "")
	preview := ""
	if r.preview != "" {
		preview = `<div style="display:none;font-size:1px;color:#ffffff;line-height:1px;max-height:0px;max-width:0px;opacity:0;overflow:hidden;mso-hide:all;">` + r.preview + `</div>`
	}
	return `<!doctype html><html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" lang="` + esc(r.lang) + `" dir="` + esc(r.dir) + `"><head><title>` + esc(r.title) + `</title><meta charset="utf-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1"><!--[if mso]><xml><o:OfficeDocumentSettings><o:AllowPNG/><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml><![endif]-->` + fonts + `<style type="text/css">` + css.String() + `</style></head><body style="word-spacing:normal;` + style(selectAttrs(a, "background-color")) + `">` + preview + `<div role="article" aria-roledescription="email" lang="` + esc(r.lang) + `" style="` + style(selectAttrs(a, "background-color")) + `">` + content + `</div></body></html>`, nil
}

func (r *renderer) checkFeatures(n *node) error {
	a := r.attrs(n)
	for _, key := range []string{"gutter", "gap", "inner-background-color", "inner-border", "inner-border-top", "inner-border-right", "inner-border-bottom", "inner-border-left", "inner-border-radius"} {
		if a[key] != "" {
			return fmt.Errorf("mjml: attribute %s is not supported", key)
		}
	}
	if n.tag == "mj-navbar" && a["hamburger"] == "hamburger" {
		return fmt.Errorf("mjml: hamburger navigation is not supported")
	}
	if n.tag == "mj-social-element" && a["name"] != "" && a["src"] == "" {
		return fmt.Errorf("mjml: social elements require an explicit src; built-in network icons are not supported")
	}
	for _, c := range n.children {
		if err := r.checkFeatures(c); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) head(n *node) error {
	for _, c := range n.children {
		switch c.tag {
		case "mj-title":
			r.title = html.UnescapeString(strings.TrimSpace(c.content))
		case "mj-preview":
			r.preview = c.content
		case "mj-style":
			if c.attrs["inline"] == "inline" {
				return fmt.Errorf("mjml: mj-style inline is not supported")
			}
			r.styles = append(r.styles, c.content)
		case "mj-raw":
			r.fonts = append(r.fonts, c.content)
		case "mj-font":
			r.fonts = append(r.fonts, `<link href="`+esc(c.attrs["href"])+`" rel="stylesheet" type="text/css">`)
		case "mj-breakpoint":
			if length(c.attrs["width"], 0) <= 0 {
				return fmt.Errorf("mjml: invalid breakpoint")
			}
			r.breakpoint = c.attrs["width"]
		case "mj-attributes":
			for _, d := range c.children {
				a := attributes{}
				merge(a, d.attrs)
				switch d.tag {
				case "mj-all":
					merge(r.all, a)
				case "mj-class":
					if a["name"] == "" {
						return fmt.Errorf("mjml: mj-class needs a name")
					}
					name := a["name"]
					delete(a, "name")
					r.classes[name] = a
				default:
					if r.defaults[d.tag] == nil {
						r.defaults[d.tag] = attributes{}
					}
					merge(r.defaults[d.tag], a)
				}
			}
		}
	}
	return nil
}

var builtin = map[string]attributes{
	"mj-body":    {"width": "600px"},
	"mj-section": {"padding": "20px 0", "direction": "ltr", "text-align": "center"}, "mj-wrapper": {"padding": "20px 0"},
	"mj-column": {"vertical-align": "top", "direction": "ltr"}, "mj-group": {"vertical-align": "top", "direction": "ltr"},
	"mj-text":    {"align": "left", "font-family": "Ubuntu, Helvetica, Arial, sans-serif", "font-size": "13px", "line-height": "1", "color": "#000000", "padding": "10px 25px"},
	"mj-button":  {"align": "center", "background-color": "#414141", "color": "#ffffff", "border-radius": "3px", "font-family": "Ubuntu, Helvetica, Arial, sans-serif", "font-size": "13px", "line-height": "120%", "font-weight": "normal", "inner-padding": "10px 25px", "padding": "10px 25px", "text-decoration": "none", "target": "_blank"},
	"mj-image":   {"align": "center", "padding": "10px 25px", "height": "auto", "target": "_blank"},
	"mj-divider": {"border-color": "#000000", "border-style": "solid", "border-width": "4px", "padding": "10px 25px", "width": "100%"},
	"mj-spacer":  {"height": "20px"}, "mj-table": {"align": "left", "color": "#000000", "font-family": "Ubuntu, Helvetica, Arial, sans-serif", "font-size": "13px", "line-height": "22px", "padding": "10px 25px", "width": "100%", "cellpadding": "0", "cellspacing": "0"},
	"mj-navbar": {"align": "center", "padding": "10px 25px"}, "mj-navbar-link": {"color": "#000000", "font-family": "Ubuntu, Helvetica, Arial, sans-serif", "font-size": "13px", "padding": "15px 10px", "text-decoration": "none", "text-transform": "uppercase", "target": "_blank"},
	"mj-social": {"align": "center", "padding": "10px 25px", "mode": "horizontal"}, "mj-social-element": {"color": "#333333", "font-size": "13px", "font-family": "Ubuntu, Helvetica, Arial, sans-serif", "padding": "4px", "icon-size": "20px", "border-radius": "3px", "target": "_blank", "text-decoration": "none"},
}

func (r *renderer) attrs(n *node) attributes {
	a := attributes{}
	merge(a, builtin[n.tag])
	merge(a, r.all)
	merge(a, r.defaults[n.tag])
	for _, c := range strings.Fields(n.attrs["mj-class"]) {
		merge(a, r.classes[c])
	}
	merge(a, n.attrs)
	return a
}
func merge(a attributes, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}
func esc(s string) string { return html.EscapeString(s) }
func sortedKeys[M ~map[string]string](m M) []string {
	k := make([]string, 0, len(m))
	for s := range m {
		k = append(k, s)
	}
	sort.Strings(k)
	return k
}
func style(a attributes) string {
	var b strings.Builder
	for _, k := range sortedKeys(a) {
		if a[k] != "" {
			b.WriteString(k + ":" + a[k] + ";")
		}
	}
	return esc(b.String())
}
func selectAttrs(a attributes, keys ...string) attributes {
	b := attributes{}
	for _, k := range keys {
		if a[k] != "" {
			b[k] = a[k]
		}
	}
	return b
}
func length(s string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
	if err != nil {
		return fallback
	}
	return v
}
func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func px(v float64) string     { return number(v) + "px" }
func dimension(s string, parent float64) float64 {
	if strings.HasSuffix(s, "%") {
		return length(strings.TrimSuffix(s, "%"), 100) * parent / 100
	}
	return length(s, parent)
}
func padding(a attributes, side string, parent float64) float64 {
	if v, ok := a["padding-"+side]; ok {
		return dimension(v, parent)
	}
	p := strings.Fields(a["padding"])
	if len(p) == 0 || len(p) > 4 {
		return 0
	}
	i := 0
	switch side {
	case "top":
		i = 0
	case "right":
		if len(p) > 1 {
			i = 1
		}
	case "bottom":
		if len(p) > 2 {
			i = 2
		}
	case "left":
		if len(p) > 3 {
			i = 3
		} else if len(p) > 1 {
			i = 1
		}
	}
	return dimension(p[i], parent)
}
func innerWidth(a attributes, w float64) float64 {
	return max(0, w-padding(a, "left", w)-padding(a, "right", w))
}
func boxStyle(a attributes) attributes {
	return selectAttrs(a, "background-color", "border", "border-top", "border-bottom", "border-left", "border-right", "border-radius", "padding", "padding-top", "padding-right", "padding-bottom", "padding-left", "vertical-align", "direction")
}
func classAttr(a attributes) string {
	if a["css-class"] == "" {
		return ""
	}
	return ` class="` + esc(a["css-class"]) + `"`
}

const tableStart = `<table border="0" cellpadding="0" cellspacing="0" role="presentation" width="100%"`
const baseCSS = `#outlook a{padding:0;}body{margin:0;padding:0;-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%;}table,td{border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt;}img{border:0;height:auto;line-height:100%;outline:none;text-decoration:none;-ms-interpolation-mode:bicubic;}p{display:block;margin:13px 0;}`

func (r *renderer) children(n *node, w float64, group bool) string {
	var b strings.Builder
	count := 0
	for _, c := range n.children {
		if c.tag == "mj-column" || c.tag == "mj-group" {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	for _, c := range n.children {
		b.WriteString(r.component(c, w, count, group))
	}
	return b.String()
}

func (r *renderer) component(n *node, w float64, siblings int, inGroup bool) string {
	a := r.attrs(n)
	switch n.tag {
	case "mj-raw":
		return n.content
	case "mj-section", "mj-wrapper":
		inner := innerWidth(a, w)
		content := r.children(n, inner, false)
		if n.tag == "mj-section" {
			content = `<!--[if mso | IE]><table role="presentation" width="` + number(inner) + `"><tr><![endif]-->` + content + `<!--[if mso | IE]></tr></table><![endif]-->`
		}
		outer := selectAttrs(a, "background-color", "border-radius")
		outer["margin"] = "0px auto"
		outer["max-width"] = px(w)
		if a["background-url"] != "" {
			outer["background-image"] = "url('" + strings.ReplaceAll(a["background-url"], "'", "%27") + "')"
			outer["background-size"] = a["background-size"]
			outer["background-repeat"] = a["background-repeat"]
		}
		td := boxStyle(a)
		td["font-size"] = "0px"
		td["text-align"] = a["text-align"]
		out := `<!--[if mso | IE]><table role="presentation" align="center" width="` + number(w) + `"><tr><td><![endif]--><div` + classAttr(a) + ` style="` + style(outer) + `">` + tableStart + `><tbody><tr><td style="` + style(td) + `">` + content + `</td></tr></tbody></table></div><!--[if mso | IE]></td></tr></table><![endif]-->`
		if a["full-width"] == "full-width" {
			return tableStart + ` style="` + style(selectAttrs(a, "background-color")) + `"><tr><td>` + out + `</td></tr></table>`
		}
		return out
	case "mj-column", "mj-group":
		width := a["width"]
		if width == "" {
			width = number(100/float64(siblings)) + "%"
		}
		cw := dimension(width, w)
		key := "mj-column-per-"
		if !strings.HasSuffix(width, "%") {
			key = "mj-column-px-"
		}
		key += strings.NewReplacer(".", "-", "%", "", "px", "").Replace(width)
		r.media[key] = width
		mobile := "100%"
		if inGroup {
			mobile = number(cw/w*100) + "%"
		}
		ds := attributes{"font-size": "0px", "text-align": "left", "display": "inline-block", "vertical-align": a["vertical-align"], "direction": a["direction"], "width": mobile}
		classes := key
		if a["css-class"] != "" {
			classes += " " + a["css-class"]
		}
		var content string
		if n.tag == "mj-group" {
			content = `<!--[if mso | IE]><table role="presentation" width="` + number(cw) + `"><tr><![endif]-->` + r.children(n, innerWidth(a, cw), true) + `<!--[if mso | IE]></tr></table><![endif]-->`
		} else {
			var b strings.Builder
			for _, c := range n.children {
				if c.tag == "mj-raw" {
					b.WriteString(c.content)
					continue
				}
				ca := r.attrs(c)
				td := selectAttrs(ca, "padding", "padding-top", "padding-right", "padding-bottom", "padding-left", "vertical-align")
				td["font-size"] = "0px"
				if ca["container-background-color"] != "" {
					td["background-color"] = ca["container-background-color"]
				}
				b.WriteString(`<tr><td align="` + esc(ca["align"]) + `"` + classAttr(ca) + ` style="` + style(td) + `">` + r.component(c, innerWidth(a, cw), 1, false) + `</td></tr>`)
			}
			content = tableStart + ` style="` + style(selectAttrs(a, "background-color", "border", "border-radius", "vertical-align")) + `"><tbody>` + b.String() + `</tbody></table>`
		}
		if a["padding"] != "" || a["padding-left"] != "" || a["padding-right"] != "" || a["padding-top"] != "" || a["padding-bottom"] != "" {
			content = tableStart + `><tr><td style="` + style(boxStyle(a)) + `">` + content + `</td></tr></table>`
		}
		return `<!--[if mso | IE]><td style="vertical-align:` + esc(a["vertical-align"]) + `;width:` + px(cw) + `;" ><![endif]--><div class="` + esc(classes) + `" style="` + style(ds) + `">` + content + `</div><!--[if mso | IE]></td><![endif]-->`
	case "mj-text":
		s := typography(a)
		s["text-align"] = a["align"]
		merge(s, selectAttrs(a, "height", "background-color"))
		return `<div style="` + style(s) + `">` + n.content + `</div>`
	case "mj-button":
		s := typography(a)
		s["display"] = "inline-block"
		s["background"] = a["background-color"]
		s["padding"] = a["inner-padding"]
		s["margin"] = "0"
		s["border-radius"] = a["border-radius"]
		s["text-align"] = "center"
		s["mso-padding-alt"] = "0px"
		s["width"] = a["width"]
		link := `<p style="` + style(s) + `">` + n.content + `</p>`
		if a["href"] != "" {
			link = `<a` + linkAttrs(a) + ` style="` + style(s) + `">` + n.content + `</a>`
		}
		td := selectAttrs(a, "background-color", "border", "border-radius")
		td["mso-padding-alt"] = a["inner-padding"]
		return `<table border="0" cellpadding="0" cellspacing="0" role="presentation" style="border-collapse:separate;line-height:100%;"><tbody><tr><td align="center" bgcolor="` + esc(a["background-color"]) + `" style="` + style(td) + `">` + link + `</td></tr></tbody></table>`
	case "mj-image":
		iw := innerWidth(a, w)
		if a["width"] != "" {
			iw = min(iw, dimension(a["width"], w))
		}
		s := selectAttrs(a, "border", "border-radius", "height")
		s["display"] = "block"
		s["outline"] = "none"
		s["width"] = "100%"
		s["max-width"] = px(iw)
		img := `<img alt="` + esc(a["alt"]) + `" src="` + esc(a["src"]) + `" width="` + number(iw) + `" height="` + esc(strings.TrimSuffix(a["height"], "px")) + `" style="` + style(s) + `"`
		for _, k := range []string{"title", "srcset", "sizes"} {
			if a[k] != "" {
				img += ` ` + k + `="` + esc(a[k]) + `"`
			}
		}
		img += `>`
		if a["href"] != "" {
			img = `<a` + linkAttrs(a) + `>` + img + `</a>`
		}
		cl := ""
		if a["fluid-on-mobile"] == "true" {
			cl = ` class="mj-full-width-mobile"`
		}
		return `<table border="0" cellpadding="0" cellspacing="0" role="presentation"` + cl + ` style="border-collapse:collapse;border-spacing:0px;"><tbody><tr><td style="width:` + px(iw) + `;">` + img + `</td></tr></tbody></table>`
	case "mj-divider":
		s := attributes{"border-top": a["border-width"] + " " + a["border-style"] + " " + a["border-color"], "font-size": "1px", "margin": "0px auto", "width": a["width"]}
		return `<p style="` + style(s) + `"></p>`
	case "mj-spacer":
		return `<div style="height:` + esc(a["height"]) + `;line-height:` + esc(a["height"]) + `;">&#8202;</div>`
	case "mj-table":
		s := typography(a)
		s["width"] = a["width"]
		s["table-layout"] = a["table-layout"]
		s["border"] = a["border"]
		return `<table border="0" cellpadding="` + esc(a["cellpadding"]) + `" cellspacing="` + esc(a["cellspacing"]) + `" width="` + esc(a["width"]) + `" style="` + style(s) + `">` + n.content + `</table>`
	case "mj-navbar":
		return `<div style="text-align:` + esc(a["align"]) + `;">` + r.children(n, w, false) + `</div>`
	case "mj-navbar-link":
		s := typography(a)
		merge(s, selectAttrs(a, "padding", "padding-top", "padding-right", "padding-bottom", "padding-left"))
		s["display"] = "inline-block"
		return `<a` + linkAttrs(a) + ` style="` + style(s) + `">` + n.content + `</a>`
	case "mj-social":
		var b strings.Builder
		for _, c := range n.children {
			if c.tag == "mj-raw" {
				b.WriteString(c.content)
				continue
			}
			display := "inline-block"
			if a["mode"] == "vertical" {
				display = "block"
			}
			b.WriteString(`<div style="display:` + display + `;">` + r.component(c, w, 1, false) + `</div>`)
		}
		return `<div style="text-align:` + esc(a["align"]) + `;">` + b.String() + `</div>`
	case "mj-social-element":
		s := typography(a)
		s["padding"] = a["padding"]
		s["display"] = "inline-block"
		icon := ""
		if a["src"] != "" {
			icon = `<img src="` + esc(a["src"]) + `" alt="` + esc(a["alt"]) + `" width="` + esc(strings.TrimSuffix(a["icon-size"], "px")) + `" style="vertical-align:middle;border-radius:` + esc(a["border-radius"]) + `;background-color:` + esc(a["background-color"]) + `;"> `
		}
		return `<a` + linkAttrs(a) + ` style="` + style(s) + `">` + icon + n.content + `</a>`
	}
	return ""
}
func typography(a attributes) attributes {
	return selectAttrs(a, "font-family", "font-size", "font-weight", "font-style", "line-height", "letter-spacing", "color", "text-decoration", "text-transform")
}
func linkAttrs(a attributes) string {
	var b strings.Builder
	for _, k := range []string{"href", "target", "rel", "title", "name"} {
		if a[k] != "" {
			b.WriteString(` ` + k + `="` + esc(a[k]) + `"`)
		}
	}
	return b.String()
}
