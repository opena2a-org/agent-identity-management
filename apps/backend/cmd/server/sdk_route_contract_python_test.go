package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Python: sdk/python/aim_sdk/client.py.

var (
	// Every way client.py sends a request: the request helper, and requests
	// or a requests session called directly.
	pySiteCall  = regexp.MustCompile(`\b\w+\._make_request\(|\brequests\.(?:get|post|put|patch|delete|request)\(|\bsession\.(?:get|post|put|patch|delete|request)\(`)
	pyDefLine   = regexp.MustCompile(`(?m)^([ \t]*)(?:async[ \t]+)?(def)[ \t]+(\w+)[ \t]*\(`)
	pyIdentExpr = regexp.MustCompile(`^[A-Za-z_]\w*$`)
	pyKeywordAt = regexp.MustCompile(`^([A-Za-z_]\w*)\s*=(?:[^=]|$)`)
	pyBaseExpr  = regexp.MustCompile(`^(?:\w+\.)?(?:aim_url|base_url)(?:\.rstrip\(\s*['"]/['"]\s*\))?$`)
	pyDumpsCall = regexp.MustCompile(`^[\w.]*\bdumps\(`)
	// The functions whose calls pySiteCall finds: a call inside one that
	// passes on its caller's path is the helper at work, not a call of its own.
	pyRequestHelpers = map[string]bool{"_make_request": true}
)

type pyFunc struct {
	name   string
	params []string
	indent int
	start  int
	end    int
}

func (fn *pyFunc) has(param string) bool {
	for _, p := range fn.params {
		if p == param {
			return true
		}
	}
	return false
}

type pySource struct {
	*srcFile
	funcs []*pyFunc
}

func classifyPython(s string) []byte {
	class := make([]byte, len(s))
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '#':
			end := lineEndIndex(s, i)
			fillClass(class, i, end, srcComment)
			i = end
		case c == '\'' || c == '"':
			end := pyStringEnd(s, i)
			fillClass(class, i, end, srcString)
			i = end
		default:
			i++
		}
	}
	return class
}

func pyStringEnd(s string, i int) int {
	q := s[i]
	if triple := strings.Repeat(string(q), 3); strings.HasPrefix(s[i:], triple) {
		for j := i + 3; j < len(s); j++ {
			if s[j] == '\\' {
				j++
				continue
			}
			if strings.HasPrefix(s[j:], triple) {
				return j + 3
			}
		}
		return len(s)
	}
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(s)
}

func newPySource(rel, text string) *pySource {
	p := &pySource{srcFile: newSrcFile(rel, text, classifyPython)}
	for _, m := range p.codeMatches(pyDefLine, 2) {
		open := m[1] - 1
		fn := &pyFunc{name: text[m[6]:m[7]], indent: m[3] - m[2], start: m[0]}
		for _, item := range p.items(open, false) {
			name := strings.TrimLeft(p.str(item), "*")
			if i := strings.IndexAny(name, ":="); i >= 0 {
				name = name[:i]
			}
			if name = strings.TrimSpace(name); name != "" {
				fn.params = append(fn.params, name)
			}
		}
		fn.end = len(text)
		for pos := lineEndIndex(text, p.match[open]) + 1; pos < len(text); {
			next := lineEndIndex(text, pos)
			k := pos
			for k < next && (text[k] == ' ' || text[k] == '\t') {
				k++
			}
			if k < next && p.code(k) && k-pos <= fn.indent {
				fn.end = pos
				break
			}
			pos = next + 1
		}
		p.funcs = append(p.funcs, fn)
	}
	return p
}

// enclosing returns the functions around off, innermost first.
func (p *pySource) enclosing(off int) []*pyFunc {
	var out []*pyFunc
	for _, fn := range p.funcs {
		if fn.start <= off && off < fn.end {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start > out[j].start })
	return out
}

func (p *pySource) funcName(off int) string {
	var names []string
	for _, fn := range p.enclosing(off) {
		names = append([]string{fn.name}, names...)
	}
	if len(names) == 0 {
		return "<module>"
	}
	return strings.Join(names, ".")
}

// statementRest returns the rest of the statement that continues at i.
func (p *pySource) statementRest(i int) span {
	j := i
	for j < len(p.text) {
		if !p.code(j) {
			j++
			continue
		}
		switch c := p.text[j]; c {
		case '(', '[', '{':
			if m := p.match[j]; m > j {
				j = m + 1
				continue
			}
		case '\n':
			if k := strings.TrimRight(p.text[i:j], " \t"); !strings.HasSuffix(k, "\\") {
				return p.trim(span{i, j})
			}
		case ';':
			return p.trim(span{i, j})
		}
		j++
	}
	return p.trim(span{i, j})
}

// assignments returns the right-hand side of each "name = ..." statement before
// off in the innermost enclosing function that assigns name, the function's
// start, and whether name is instead a parameter of that function.
func (p *pySource) assignments(name string, off int) (rhs []span, scopeStart int, fn *pyFunc) {
	re := regexp.MustCompile(`(?m)^[ \t]*(` + regexp.QuoteMeta(name) + `)[ \t]*(?::[^=\n]*)?=(?:[^=]|$)`)
	collect := func(from, to int, column0 bool) []span {
		var out []span
		for _, m := range re.FindAllStringSubmatchIndex(p.text[from:to], -1) {
			at := from + m[2]
			if !p.code(at) || p.depth[at] != 0 || (column0 && at != from+m[0]) {
				continue
			}
			eq := strings.IndexByte(p.text[at:], '=') + at
			out = append(out, p.statementRest(eq+1))
		}
		return out
	}
	for _, f := range p.enclosing(off) {
		if found := collect(f.start, off, false); len(found) > 0 {
			return found, f.start, nil
		}
		if f.has(name) {
			return nil, f.start, f
		}
	}
	return collect(0, len(p.text), true), 0, nil
}

// literal reads a span made of string literals only ("a" f"{b}" ...).
func (p *pySource) literal(s span) (pieces []string, fstring []bool, ok bool) {
	pieces, fstring, _, ok = p.literalAt(s)
	return pieces, fstring, ok
}

func (p *pySource) literalAt(s span) (pieces []string, fstring []bool, starts []int, ok bool) {
	i := s.a
	for i < s.b {
		for i < s.b && (isSpaceByte(p.text[i]) || p.class[i] == srcComment || p.text[i] == '\\') {
			i++
		}
		if i >= s.b {
			break
		}
		j := i
		for j < s.b && p.code(j) && strings.IndexByte("rRbBfFuU", p.text[j]) >= 0 {
			j++
		}
		if j >= s.b || p.class[j] != srcString {
			return nil, nil, nil, false
		}
		end := pyStringEnd(p.text, j)
		q := 1
		if strings.HasPrefix(p.text[j:], `"""`) || strings.HasPrefix(p.text[j:], `'''`) {
			q = 3
		}
		pieces = append(pieces, p.text[j+q:end-q])
		fstring = append(fstring, strings.ContainsAny(p.text[i:j], "fF"))
		starts = append(starts, j)
		i = end
	}
	return pieces, fstring, starts, len(pieces) > 0
}

// stringOf reads s as a string template; off is where names are looked up.
func (p *pySource) stringOf(s span, off, depth int) (string, error) {
	if depth > 10 {
		return "", fmt.Errorf("the value is built too indirectly")
	}
	s = p.unwrap(s)
	text := p.str(s)
	if pieces, fstring, starts, ok := p.literalAt(s); ok {
		for _, start := range starts {
			p.read[start] = true
		}
		var b strings.Builder
		for i, piece := range pieces {
			if !fstring[i] {
				b.WriteString(piece)
				continue
			}
			for j := 0; j < len(piece); j++ {
				switch {
				case strings.HasPrefix(piece[j:], "{{"):
					b.WriteByte('{')
					j++
				case strings.HasPrefix(piece[j:], "}}"):
					b.WriteByte('}')
					j++
				case piece[j] == '{':
					end := closingBraceIn(piece, j)
					b.WriteString(p.field(piece[j+1:end], off, depth))
					j = end
				default:
					b.WriteByte(piece[j])
				}
			}
		}
		return b.String(), nil
	}
	if pyIdentExpr.MatchString(text) {
		rhs, _, param := p.assignments(text, off)
		switch {
		case len(rhs) > 0:
			last := rhs[len(rhs)-1]
			return p.stringOf(last, last.a, depth+1)
		case param != nil:
			return "{" + text + "}", nil
		}
		return "", fmt.Errorf("%s is not assigned a string before the call", text)
	}
	if parts := p.splitTop(s, '+'); len(parts) > 1 {
		var b strings.Builder
		for _, part := range parts {
			piece, err := p.stringOf(part, off, depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(piece)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("cannot read %q as a URL", shortText(text))
}

// field reads one {expression} of an f-string.
func (p *pySource) field(expr string, off, depth int) string {
	for i, d := 0, 0; i < len(expr); i++ {
		switch c := expr[i]; c {
		case '\'', '"':
			for i++; i < len(expr) && expr[i] != c; i++ {
			}
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		case '!', ':':
			if d == 0 && !(c == '!' && i+1 < len(expr) && expr[i+1] == '=') {
				expr = expr[:i]
			}
		}
	}
	expr = strings.TrimSpace(expr)
	if pyBaseExpr.MatchString(expr) {
		return sdkBaseURL
	}
	if pyIdentExpr.MatchString(expr) {
		if rhs, _, _ := p.assignments(expr, off); len(rhs) > 0 {
			last := rhs[len(rhs)-1]
			if s, err := p.stringOf(last, last.a, depth+1); err == nil && !strings.Contains(s, "?") {
				return s
			}
		}
	}
	return placeholder(expr)
}

// membersOf reads the top-level members of the JSON body s holds.
func (p *pySource) membersOf(s span, off, depth int) (map[string]bool, error) {
	if depth > 10 {
		return nil, fmt.Errorf("the body is built too indirectly")
	}
	s = p.unwrap(s)
	text := p.str(s)
	out := map[string]bool{}
	switch {
	case text == "None":
		return out, nil
	case strings.HasPrefix(text, "{") && p.match[s.a] == s.b-1:
		for _, item := range p.items(s.a, false) {
			itemText := p.str(item)
			if strings.HasPrefix(itemText, "**") {
				more, err := p.membersOf(span{item.a + 2, item.b}, off, depth+1)
				if err != nil {
					return nil, err
				}
				merge(out, more)
				continue
			}
			colon := p.topLevelIndex(item, ":")
			if colon < 0 {
				return nil, fmt.Errorf("cannot read the dict entry %q", shortText(itemText))
			}
			key, fstring, ok := p.literal(p.trim(span{item.a, colon}))
			if !ok || len(key) != 1 || fstring[0] {
				return nil, fmt.Errorf("the dict key %q is not a string literal", shortText(p.str(span{item.a, colon})))
			}
			out[key[0]] = true
		}
		return out, nil
	case pyDumpsCall.MatchString(text):
		open := s.a + strings.IndexByte(text, '(')
		if p.match[open] == s.b-1 {
			if args := p.items(open, false); len(args) > 0 {
				return p.membersOf(args[0], off, depth+1)
			}
		}
	case pyIdentExpr.MatchString(text):
		return p.nameMembers(text, off, depth)
	}
	return nil, fmt.Errorf("cannot read the body %q", shortText(text))
}

// nameMembers reads the members a dict variable holds at off: the dicts it is
// assigned (or, for a parameter, the dicts every caller passes), with the keys
// set on it and the dicts merged into it before off.
func (p *pySource) nameMembers(name string, off, depth int) (map[string]bool, error) {
	out := map[string]bool{}
	rhs, scopeStart, param := p.assignments(name, off)
	for _, r := range rhs {
		more, err := p.membersOf(r, r.a, depth+1)
		if err != nil {
			return nil, err
		}
		merge(out, more)
	}
	if len(rhs) == 0 {
		if param == nil {
			return nil, fmt.Errorf("%s is not assigned a dict before the call", name)
		}
		args, err := p.callerArguments(param, name)
		if err != nil {
			return nil, err
		}
		for _, arg := range args {
			more, err := p.membersOf(arg, arg.a, depth+1)
			if err != nil {
				return nil, err
			}
			merge(out, more)
		}
	}
	from := scopeStart
	if len(rhs) > 0 {
		from = rhs[0].a
	}
	setKey := regexp.MustCompile(`(?m)^[ \t]*(` + regexp.QuoteMeta(name) + `)\[[ \t]*['"]([^'"\n]+)['"][ \t]*\][ \t]*=(?:[^=]|$)`)
	for _, m := range setKey.FindAllStringSubmatchIndex(p.text[from:off], -1) {
		if p.code(from + m[2]) {
			out[p.text[from+m[4]:from+m[5]]] = true
		}
	}
	update := regexp.MustCompile(`(?m)^[ \t]*(` + regexp.QuoteMeta(name) + `)\.update\(`)
	for _, m := range update.FindAllStringSubmatchIndex(p.text[from:off], -1) {
		if !p.code(from + m[2]) {
			continue
		}
		args := p.items(from+m[1]-1, false)
		if len(args) != 1 {
			return nil, fmt.Errorf("cannot read %s.update(...)", name)
		}
		more, err := p.membersOf(args[0], args[0].a, depth+1)
		if err != nil {
			return nil, err
		}
		merge(out, more)
	}
	return out, nil
}

// callerArguments returns the argument every call of fn passes for param.
func (p *pySource) callerArguments(fn *pyFunc, param string) ([]span, error) {
	index := -1
	for i, name := range fn.params {
		if name == param {
			index = i
		}
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(fn.name) + `\(`)
	var out []span
	for _, m := range p.codeMatches(re, 0) {
		if strings.HasSuffix(strings.TrimRight(p.text[:m[0]], " \t"), "def") {
			continue
		}
		position := index
		if m[0] > 0 && p.text[m[0]-1] == '.' && len(fn.params) > 0 && (fn.params[0] == "self" || fn.params[0] == "cls") {
			position--
		}
		var positional []span
		var keyword span
		for _, arg := range p.items(m[1]-1, false) {
			if k := pyKeywordAt.FindStringSubmatch(p.str(arg)); k != nil {
				if k[1] == param {
					keyword = p.trim(span{arg.a + strings.IndexByte(p.str(arg), '=') + 1, arg.b})
				}
				continue
			}
			positional = append(positional, arg)
		}
		switch {
		case keyword.b > keyword.a:
			out = append(out, keyword)
		case position >= 0 && position < len(positional):
			out = append(out, positional[position])
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s is a parameter of %s, and no call of %s passes it", param, fn.name, fn.name)
	}
	return out, nil
}

func extractPythonCalls(t *testing.T, root string) sdkExtraction {
	t.Helper()
	rel := "sdk/python/aim_sdk/client.py"
	text, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // G304: a repository file
	require.NoError(t, err)
	p := newPySource(rel, string(text))

	var ex sdkExtraction
	for _, m := range p.codeMatches(pySiteCall, 0) {
		ex.sites++
		open := m[1] - 1
		callee := p.text[m[0]:open]
		line := p.line(m[0])
		function := p.funcName(m[0])
		fail := func(why string) {
			ex.unreadable = append(ex.unreadable, fmt.Sprintf("%s:%d (%s): %s", rel, line, function, why))
		}

		var positional []span
		keyword := map[string]span{}
		for _, arg := range p.items(open, false) {
			if k := pyKeywordAt.FindStringSubmatch(p.str(arg)); k != nil {
				keyword[k[1]] = p.trim(span{arg.a + strings.IndexByte(p.str(arg), '=') + 1, arg.b})
				continue
			}
			positional = append(positional, arg)
		}
		arg := func(index int, name string) (span, bool) {
			if s, ok := keyword[name]; ok {
				return s, true
			}
			if index >= 0 && index < len(positional) {
				return positional[index], true
			}
			return span{}, false
		}

		var methodArg, urlArg, bodyArg span
		var hasMethod, hasURL, hasBody bool
		method := ""
		switch {
		case strings.HasSuffix(callee, "._make_request"):
			methodArg, hasMethod = arg(0, "method")
			urlArg, hasURL = arg(1, "endpoint")
			bodyArg, hasBody = arg(2, "data")
		case strings.HasSuffix(callee, ".request"):
			methodArg, hasMethod = arg(0, "method")
			urlArg, hasURL = arg(1, "url")
		default:
			method = strings.ToUpper(callee[strings.LastIndexByte(callee, '.')+1:])
			hasMethod = true
			urlArg, hasURL = arg(0, "url")
		}
		if !strings.HasSuffix(callee, "_make_request") {
			if bodyArg, hasBody = keyword["json"]; !hasBody {
				bodyArg, hasBody = keyword["data"]
			}
		}
		if !hasMethod || !hasURL {
			fail("the call names no method or no URL")
			continue
		}

		forwardedMethod := false
		if method == "" {
			if pieces, fstring, ok := p.literal(methodArg); ok && len(pieces) == 1 && !fstring[0] {
				method = strings.ToUpper(pieces[0])
			} else if name := p.str(methodArg); pyIdentExpr.MatchString(name) {
				if _, _, param := p.assignments(name, methodArg.a); param != nil {
					forwardedMethod = true
				}
			}
			if method == "" && !forwardedMethod {
				fail("cannot read the method " + shortText(p.str(methodArg)))
				continue
			}
		}

		template, err := p.stringOf(urlArg, urlArg.a, 0)
		if err != nil {
			fail(err.Error())
			continue
		}
		path, forwarded, err := sdkPath(template)
		if err != nil {
			fail(err.Error())
			continue
		}
		if forwarded {
			if enclosing := p.enclosing(m[0]); len(enclosing) == 0 || !pyRequestHelpers[enclosing[0].name] {
				fail("passes on a path from its caller, but is not a request helper whose calls the test reads")
				continue
			}
			ex.forwarded = append(ex.forwarded, fmt.Sprintf("%s:%d (%s)", rel, line, function))
			continue
		}
		if forwardedMethod {
			fail("passes on a method from its caller for the fixed path " + path)
			continue
		}

		members := map[string]bool{}
		if hasBody {
			if members, err = p.membersOf(bodyArg, bodyArg.a, 0); err != nil {
				fail(err.Error())
				continue
			}
		}
		ex.calls = append(ex.calls, sdkCall{file: rel, line: line, function: function, method: method, path: path, members: sortedKeys(members)})
	}
	ex.unreadable = append(ex.unreadable, p.unreadServerPaths()...)
	return ex
}
