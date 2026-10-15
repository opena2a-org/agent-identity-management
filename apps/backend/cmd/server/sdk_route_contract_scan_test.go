package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Source scanning shared by the Python and TypeScript readers.

const (
	srcCode byte = iota
	srcString
	srcComment
)

// sdkBaseURL stands for the server's address in a URL template; it is
// stripped, and must be what a URL starts with.
const sdkBaseURL = "\x00base\x00"

type span struct{ a, b int }

// srcFile is a source text with each byte classed as code, string or comment,
// the partner of each bracket in code, and how many brackets enclose it.
type srcFile struct {
	rel   string
	text  string
	class []byte
	match []int
	depth []int
	read  map[int]bool // the string literals read as part of a call's URL
}

func newSrcFile(rel, text string, classify func(string) []byte) *srcFile {
	f := &srcFile{rel: rel, text: text, class: classify(text), match: make([]int, len(text)), depth: make([]int, len(text)), read: map[int]bool{}}
	var stack []int
	for i := 0; i < len(text); i++ {
		f.match[i] = -1
		f.depth[i] = len(stack)
		if f.class[i] != srcCode {
			continue
		}
		switch text[i] {
		case '(', '[', '{':
			stack = append(stack, i)
		case ')', ']', '}':
			if n := len(stack); n > 0 {
				f.match[stack[n-1]] = i
				f.match[i] = stack[n-1]
				stack = stack[:n-1]
				f.depth[i] = len(stack)
			}
		}
	}
	return f
}

func (f *srcFile) code(i int) bool { return i >= 0 && i < len(f.text) && f.class[i] == srcCode }
func (f *srcFile) line(i int) int  { return strings.Count(f.text[:i], "\n") + 1 }
func (f *srcFile) str(s span) string {
	return f.text[s.a:s.b]
}

// codeMatches returns every match of re whose group g starts in code.
func (f *srcFile) codeMatches(re *regexp.Regexp, g int) [][]int {
	var out [][]int
	for _, m := range re.FindAllStringSubmatchIndex(f.text, -1) {
		if f.code(m[2*g]) {
			out = append(out, m)
		}
	}
	return out
}

func (f *srcFile) trim(s span) span {
	for s.a < s.b && (isSpaceByte(f.text[s.a]) || f.class[s.a] == srcComment) {
		s.a++
	}
	for s.b > s.a && (isSpaceByte(f.text[s.b-1]) || f.class[s.b-1] == srcComment) {
		s.b--
	}
	return s
}

// unwrap trims s and removes parentheses that enclose all of it.
func (f *srcFile) unwrap(s span) span {
	s = f.trim(s)
	for s.b-s.a >= 2 && f.code(s.a) && f.text[s.a] == '(' && f.match[s.a] == s.b-1 {
		s = f.trim(span{s.a + 1, s.b - 1})
	}
	return s
}

// items splits the bracketed list that opens at open into its top-level
// items. With angles, <...> nests as well (TypeScript type arguments).
func (f *srcFile) items(open int, angles bool) []span {
	end := f.match[open]
	if end < 0 {
		return nil
	}
	var out []span
	start, angle := open+1, 0
	for i := open + 1; i < end; i++ {
		if !f.code(i) {
			continue
		}
		switch c := f.text[i]; {
		case c == '(' || c == '[' || c == '{':
			i = f.match[i]
		case angles && c == '<':
			angle++
		case angles && c == '>' && f.text[i-1] != '=' && angle > 0:
			angle--
		case c == ',' && angle == 0:
			if s := f.trim(span{start, i}); s.a < s.b {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := f.trim(span{start, end}); s.a < s.b {
		out = append(out, s)
	}
	return out
}

// splitTop splits s at each top-level occurrence of sep in code.
func (f *srcFile) splitTop(s span, sep byte) []span {
	var out []span
	start := s.a
	for i := s.a; i < s.b; i++ {
		if !f.code(i) {
			continue
		}
		switch f.text[i] {
		case '(', '[', '{':
			if m := f.match[i]; m > i && m < s.b {
				i = m
			}
		case sep:
			out = append(out, f.unwrap(span{start, i}))
			start = i + 1
		}
	}
	return append(out, f.unwrap(span{start, s.b}))
}

// topLevelIndex returns the first top-level index of sep in s, or -1.
func (f *srcFile) topLevelIndex(s span, sep string) int {
	for i := s.a; i+len(sep) <= s.b; i++ {
		if !f.code(i) {
			continue
		}
		switch f.text[i] {
		case '(', '[', '{':
			if m := f.match[i]; m > i && m < s.b {
				i = m
				continue
			}
		}
		if strings.HasPrefix(f.text[i:], sep) {
			return i
		}
	}
	return -1
}

// unreadServerPaths returns a line for each string literal in code that looks
// like a server path (it holds "/api/" and no whitespace or escape, so it is not
// a message or a signed string), is not an argument of an error helper, and
// that no call the test read took its URL from: a request sent some way the
// call-site patterns do not find.
func (f *srcFile) unreadServerPaths() []string {
	var out []string
	for i := 0; i < len(f.text); i++ {
		if f.class[i] != srcString || (i > 0 && f.class[i-1] == srcString) {
			continue
		}
		j := i
		for j < len(f.text) && f.class[j] == srcString {
			j++
		}
		if lit := f.text[i:j]; strings.Contains(lit, "/api/") && !strings.ContainsAny(lit, " \t\n\\") && !f.read[i] && !f.inErrorCall(i) {
			out = append(out, fmt.Sprintf("%s:%d: %s names a server path, but no call the test reads uses it", f.rel, f.line(i), lit))
		}
		i = j - 1
	}
	return out
}

// inErrorCall reports whether i is inside the arguments of a call whose name
// says it builds an error, such as networkError(...): a URL named there is part
// of a message.
func (f *srcFile) inErrorCall(i int) bool {
	for open := i - 1; open >= 0; open-- {
		if !f.code(open) || f.text[open] != '(' || f.match[open] < i {
			continue
		}
		j := open
		for j > 0 && (isSpaceByte(f.text[j-1]) || f.text[j-1] == '.' || f.text[j-1] == '_' || isTSIdentByte(f.text[j-1])) {
			j--
		}
		return strings.Contains(strings.ToLower(f.text[j:open]), "error")
	}
	return false
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func lineEndIndex(s string, i int) int {
	if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
		return i + j
	}
	return len(s)
}

func fillClass(class []byte, a, b int, v byte) {
	for i := a; i < b && i < len(class); i++ {
		class[i] = v
	}
}

// closingBraceIn returns the index of the brace closing the one at i in a
// plain string (an f-string or template field), skipping quoted text.
func closingBraceIn(s string, i int) int {
	depth := 0
	for j := i; j < len(s); j++ {
		switch c := s[j]; c {
		case '\'', '"':
			for j++; j < len(s) && s[j] != c; j++ {
				if s[j] == '\\' {
					j++
				}
			}
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(s) - 1
}

var placeholderChars = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// placeholder names a path parameter after the expression that fills it.
func placeholder(expr string) string {
	name := strings.Trim(placeholderChars.ReplaceAllString(expr, "_"), "_")
	if name == "" {
		name = "param"
	}
	return "{" + name + "}"
}

var forwardedPath = regexp.MustCompile(`^\{\w+\}$`)

// sdkPath turns a URL template into a path: the server's address stripped and
// the query string cut. forwarded is true when the whole path is one parameter
// of the enclosing function, which passes on its caller's path.
func sdkPath(template string) (path string, forwarded bool, err error) {
	path = strings.TrimPrefix(template, sdkBaseURL)
	if strings.Contains(path, sdkBaseURL) {
		return "", false, fmt.Errorf("the server's address is not at the start of %q", template)
	}
	if forwardedPath.MatchString(path) {
		return path, true, nil
	}
	if !strings.HasPrefix(path, "/") {
		return "", false, fmt.Errorf("%q is not a path on the server", strings.ReplaceAll(template, sdkBaseURL, "{base}"))
	}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return path, false, nil
}

func merge(into, from map[string]bool) {
	for k := range from {
		into[k] = true
	}
}

func shortText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}
