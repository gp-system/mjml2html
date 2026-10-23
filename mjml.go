// Package mjml2html compiles MJML email templates into responsive HTML in pure Go.
package mjml2html

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"path"
)

// Options configures one render. The zero value is ready to use.
type Options struct {
	// IncludeFS explicitly enables mj-include in this filesystem. Paths are relative
	// to it; absolute paths and parent traversal are rejected.
	IncludeFS fs.FS
	// TemplateFuncs supplies functions to RenderTemplate.
	TemplateFuncs template.FuncMap
}

// Render compiles an MJML document. At most one Options value may be supplied.
// Invalid structure and unsupported components return an error and no HTML.
func Render(source string, options ...Options) (string, error) {
	if len(options) > 1 {
		return "", fmt.Errorf("mjml: expected at most one Options value")
	}
	var opts Options
	if len(options) == 1 {
		opts = options[0]
	}
	root, err := parse(source)
	if err != nil {
		return "", err
	}
	if err = expandIncludes(root, opts.IncludeFS, ".", 0); err != nil {
		return "", err
	}
	r := newRenderer()
	return r.render(root)
}

// RenderTemplate executes html/template first (escaping untrusted data), then
// compiles the resulting MJML. Use template.HTML only for trusted rich content.
func RenderTemplate(source string, data any, options ...Options) (string, error) {
	if len(options) > 1 {
		return "", fmt.Errorf("mjml: expected at most one Options value")
	}
	var opts Options
	if len(options) == 1 {
		opts = options[0]
	}
	t, err := template.New("mjml").Funcs(opts.TemplateFuncs).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("mjml template: %w", err)
	}
	var b bytes.Buffer
	if err = t.Execute(&b, data); err != nil {
		return "", fmt.Errorf("mjml template: %w", err)
	}
	return Render(b.String(), opts)
}

func expandIncludes(n *node, files fs.FS, dir string, depth int) error {
	if depth > 32 {
		return fmt.Errorf("mjml: include depth exceeds 32 (possible cycle)")
	}
	var children []*node
	for _, c := range n.children {
		if c.tag != "mj-include" {
			if err := expandIncludes(c, files, dir, depth); err != nil {
				return err
			}
			children = append(children, c)
			continue
		}
		if files == nil {
			return fmt.Errorf("mjml: mj-include requires Options.IncludeFS")
		}
		p := c.attrs["path"]
		if !fs.ValidPath(p) {
			return fmt.Errorf("mjml: invalid include path %q", p)
		}
		name := path.Join(dir, p)
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("mjml include %s: %w", name, err)
		}
		switch c.attrs["type"] {
		case "html":
			children = append(children, &node{tag: "mj-raw", content: string(data), attrs: map[string]string{}})
		case "css":
			if c.attrs["css-inline"] == "inline" {
				return fmt.Errorf("mjml: inline CSS includes are not supported")
			}
			children = append(children, &node{tag: "mj-style", content: string(data), attrs: map[string]string{}})
		case "", "mjml":
			part, err := parse("<mjml>" + string(data) + "</mjml>")
			if err != nil {
				return fmt.Errorf("mjml include %s: %w", name, err)
			}
			if err = expandIncludes(part, files, path.Dir(name), depth+1); err != nil {
				return err
			}
			children = append(children, part.children...)
		default:
			return fmt.Errorf("mjml: unsupported include type %q", c.attrs["type"])
		}
	}
	n.children = children
	return nil
}
