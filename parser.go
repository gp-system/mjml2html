package mjml2html

import (
	"fmt"
	"html"
	"strings"
)

type node struct {
	tag      string
	attrs    map[string]string
	children []*node
	content  string
}

var endingTags = map[string]bool{
	"mj-text":           true,
	"mj-button":         true,
	"mj-table":          true,
	"mj-raw":            true,
	"mj-style":          true,
	"mj-title":          true,
	"mj-preview":        true,
	"mj-navbar-link":    true,
	"mj-social-element": true,
}

// parse keeps ending-tag contents verbatim: email HTML is not necessarily XML.
func parse(source string) (*node, error) {
	root := &node{tag: "#document"}
	stack := []*node{root}
	for i := 0; i < len(source); {
		if source[i] != '<' {
			j := strings.IndexByte(source[i:], '<')
			if j < 0 {
				j = len(source) - i
			}
			if strings.TrimSpace(source[i:i+j]) != "" {
				return nil, fmt.Errorf("mjml: unexpected text outside an ending tag at byte %d", i)
			}
			i += j
			continue
		}
		if strings.HasPrefix(source[i:], "<!--") {
			j := strings.Index(source[i+4:], "-->")
			if j < 0 {
				return nil, fmt.Errorf("mjml: unclosed comment")
			}
			i += j + 7
			continue
		}
		end, err := tagEnd(source, i)
		if err != nil {
			return nil, err
		}
		token := strings.TrimSpace(source[i+1 : end])
		i = end + 1
		if strings.HasPrefix(token, "?") {
			continue
		}
		if strings.HasPrefix(token, "/") {
			name := strings.TrimSpace(token[1:])
			if len(stack) == 1 || stack[len(stack)-1].tag != name {
				return nil, fmt.Errorf("mjml: mismatched closing tag </%s>", name)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		self := strings.HasSuffix(token, "/")
		token = strings.TrimSpace(strings.TrimSuffix(token, "/"))
		name, attrs, err := parseTag(token)
		if err != nil {
			return nil, err
		}
		n := &node{tag: name, attrs: attrs}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, n)
		if self {
			continue
		}
		if endingTags[name] {
			close := "</" + name + ">"
			j := strings.Index(source[i:], close)
			if j < 0 {
				return nil, fmt.Errorf("mjml: missing %s", close)
			}
			n.content = source[i : i+j]
			i += j + len(close)
			continue
		}
		stack = append(stack, n)
		if len(stack) > 256 {
			return nil, fmt.Errorf("mjml: nesting exceeds 256 levels")
		}
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("mjml: unclosed <%s>", stack[len(stack)-1].tag)
	}
	if len(root.children) != 1 || root.children[0].tag != "mjml" {
		return nil, fmt.Errorf("mjml: expected a single <mjml> root")
	}
	return root.children[0], nil
}

func tagEnd(s string, start int) (int, error) {
	var quote byte
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		}
		if c == '>' {
			return i, nil
		}
	}
	return 0, fmt.Errorf("mjml: unclosed tag at byte %d", start)
}

func parseTag(s string) (string, map[string]string, error) {
	i := 0
	for i < len(s) && !space(s[i]) {
		i++
	}
	name := s[:i]
	attrs := map[string]string{}
	if !strings.HasPrefix(name, "mj-") && name != "mjml" {
		return "", nil, fmt.Errorf("mjml: unexpected element <%s>", name)
	}
	for i < len(s) {
		for i < len(s) && space(s[i]) {
			i++
		}
		if i == len(s) {
			break
		}
		start := i
		for i < len(s) && !space(s[i]) && s[i] != '=' {
			i++
		}
		key := s[start:i]
		for i < len(s) && space(s[i]) {
			i++
		}
		if key == "" || i == len(s) || s[i] != '=' {
			return "", nil, fmt.Errorf("mjml: attribute %q requires a quoted value", key)
		}
		i++
		for i < len(s) && space(s[i]) {
			i++
		}
		if i == len(s) || (s[i] != '\'' && s[i] != '"') {
			return "", nil, fmt.Errorf("mjml: attribute %s requires a quoted value", key)
		}
		q := s[i]
		i++
		start = i
		for i < len(s) && s[i] != q {
			i++
		}
		if i == len(s) {
			return "", nil, fmt.Errorf("mjml: unclosed attribute %s", key)
		}
		if _, ok := attrs[key]; ok {
			return "", nil, fmt.Errorf("mjml: duplicate attribute %s", key)
		}
		attrs[key] = html.UnescapeString(s[start:i])
		i++
	}
	return name, attrs, nil
}
func space(c byte) bool { return c == ' ' || c == '\n' || c == '\r' || c == '\t' }
