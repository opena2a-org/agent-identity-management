package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TypeScript: every sdk/typescript/src/**/*Client.ts, and auth/oauth.ts, which
// sends the token request for AIMClient.

var (
	// Every way the clients send a request: their request helpers, and fetch.
	tsSiteCall    = regexp.MustCompile(`\bthis\.(?:host\.)?(request|requestAsAgent|send|get|post|put|delete|patch)\s*[<(]|\b(fetch)\s*\(`)
	tsIdentExpr   = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	tsBaseExpr    = regexp.MustCompile(`^(?:this\.)?(?:config\.)?baseUrl$`)
	tsObjectKey   = regexp.MustCompile(`^(?:(?:readonly\s+)?['"]([^'"]+)['"]|(?:readonly\s+)?([A-Za-z_$][\w$]*))\s*\??\s*(?::|\(|$)`)
	tsImportNames = regexp.MustCompile(`import\s+(?:type\s+)?\{([^}]*)\}\s*from\s*['"](\.[^'"]+)['"]`)
	tsExportAll   = regexp.MustCompile(`export\s+\*\s+from\s*['"](\.[^'"]+)['"]`)
	tsStringCall  = regexp.MustCompile(`^JSON\.stringify\(`)
	tsCallExpr    = regexp.MustCompile(`^([A-Za-z_$][\w$]*)\s*(?:<[^()]*>)?\(`)
	// The helpers whose calls tsSiteCall finds; see pyRequestHelpers.
	tsRequestHelpers = map[string]bool{"request": true, "requestAsAgent": true, "send": true, "get": true, "post": true, "put": true, "delete": true, "patch": true}
	tsControlWords   = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "with": true, "return": true, "typeof": true, "await": true}
)

type tsFunc struct {
	name   string
	params map[string]string // parameter name → declared type, "" when none
	body   span
}

type tsSource struct {
	*srcFile
	abs   string
	funcs []*tsFunc
	prog  *tsProgram
}

// tsProgram opens TypeScript files once, for the constants, functions and
// types a client imports.
type tsProgram struct {
	root  string
	files map[string]*tsSource
}

func (prog *tsProgram) open(abs string) (*tsSource, error) {
	if src, ok := prog.files[abs]; ok {
		return src, nil
	}
	text, err := os.ReadFile(abs) //nolint:gosec // G304: a repository file
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(prog.root, abs)
	src := &tsSource{srcFile: newSrcFile(filepath.ToSlash(rel), string(text), classifyTypeScript), abs: abs, prog: prog}
	src.funcs = tsFunctions(src.srcFile)
	prog.files[abs] = src
	return src, nil
}

func classifyTypeScript(s string) []byte {
	class := make([]byte, len(s))
	prev, word := byte(0), ""
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			end := lineEndIndex(s, i)
			fillClass(class, i, end, srcComment)
			i = end
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := len(s)
			if j := strings.Index(s[i+2:], "*/"); j >= 0 {
				end = i + 2 + j + 2
			}
			fillClass(class, i, end, srcComment)
			i = end
		case c == '\'' || c == '"':
			end := tsQuoteEnd(s, i)
			fillClass(class, i, end, srcString)
			i, prev, word = end, 'a', ""
		case c == '`':
			end := tsTemplateEnd(s, i)
			fillClass(class, i, end, srcString)
			i, prev, word = end, 'a', ""
		case c == '/' && tsRegexMayStart(prev, word):
			end := tsRegexEnd(s, i)
			fillClass(class, i, end, srcString)
			i, prev, word = end, 'a', ""
		case isTSIdentByte(c):
			j := i
			for j < len(s) && isTSIdentByte(s[j]) {
				j++
			}
			word, prev, i = s[i:j], s[j-1], j
		default:
			if !isSpaceByte(c) {
				prev, word = c, ""
			}
			i++
		}
	}
	return class
}

func isTSIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func tsRegexMayStart(prev byte, word string) bool {
	switch word {
	case "return", "typeof", "case", "in", "of", "delete", "void", "throw", "new", "yield", "await":
		return true
	case "":
		return prev == 0 || strings.IndexByte("(,=:[!&|?{;+-*%<>~^", prev) >= 0
	}
	return false
}

func tsQuoteEnd(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case s[i]:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(s)
}

func tsTemplateEnd(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '`':
			return j + 1
		case strings.HasPrefix(s[j:], "${"):
			j = tsInterpolationEnd(s, j+2) - 1
		}
	}
	return len(s)
}

// tsInterpolationEnd returns the index after the } that closes a ${ whose
// expression starts at i.
func tsInterpolationEnd(s string, i int) int {
	depth := 1
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '\'', '"':
			j = tsQuoteEnd(s, j) - 1
		case '`':
			j = tsTemplateEnd(s, j) - 1
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return j + 1
			}
		}
	}
	return len(s)
}

func tsRegexEnd(s string, i int) int {
	inClass := false
	for j := i + 1; j < len(s); j++ {
		switch c := s[j]; {
		case c == '\\':
			j++
		case c == '\n':
			return j
		case inClass:
			inClass = c != ']'
		case c == '[':
			inClass = true
		case c == '/':
			j++
			for j < len(s) && isTSIdentByte(s[j]) {
				j++
			}
			return j
		}
	}
	return len(s)
}

// tsFunctions finds every function, method and arrow function with a
// parenthesised parameter list, with its parameters and body.
func tsFunctions(f *srcFile) []*tsFunc {
	var out []*tsFunc
	for open := 0; open < len(f.text); open++ {
		if !f.code(open) || f.text[open] != '(' || f.match[open] < 0 {
			continue
		}
		name := tsWordBefore(f, open)
		if tsControlWords[name] {
			continue
		}
		i := tsSkipSpace(f, f.match[open]+1)
		if i < len(f.text) && f.text[i] == ':' && f.code(i) {
			if i = tsSkipReturnType(f, i+1); i < 0 {
				continue
			}
		}
		arrow := false
		if strings.HasPrefix(f.text[i:], "=>") {
			arrow = true
			i = tsSkipSpace(f, i+2)
		}
		var body span
		switch {
		case i < len(f.text) && f.text[i] == '{' && f.code(i):
			if !arrow && (name == "" || name == "async") {
				continue
			}
			body = span{i, f.match[i] + 1}
		case arrow:
			body = span{i, tsExpressionEnd(f, i)}
		default:
			continue
		}
		if arrow {
			name = tsArrowName(f, open)
		}
		fn := &tsFunc{name: name, params: map[string]string{}, body: body}
		for _, item := range f.items(open, true) {
			text := strings.TrimSpace(f.str(item))
			for _, modifier := range []string{"public ", "private ", "protected ", "readonly ", "..."} {
				text = strings.TrimSpace(strings.TrimPrefix(text, modifier))
			}
			j := 0
			for j < len(text) && isTSIdentByte(text[j]) {
				j++
			}
			if j == 0 {
				continue
			}
			paramName, rest := text[:j], strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text[j:]), "?"))
			typ := ""
			if strings.HasPrefix(rest, ":") {
				typ = strings.TrimSpace(tsCutDefault(rest[1:]))
			}
			fn.params[paramName] = typ
		}
		out = append(out, fn)
	}
	return out
}

func tsSkipSpace(f *srcFile, i int) int {
	for i < len(f.text) && (isSpaceByte(f.text[i]) || f.class[i] == srcComment) {
		i++
	}
	return i
}

// tsWordBefore returns the identifier before the ( at open, past any type
// parameters: "send" for "send<T>(".
func tsWordBefore(f *srcFile, open int) string {
	j := open - 1
	for j >= 0 && isSpaceByte(f.text[j]) {
		j--
	}
	if j >= 0 && f.text[j] == '>' {
		for depth := 0; j >= 0; j-- {
			if f.text[j] == '>' {
				depth++
			} else if f.text[j] == '<' {
				if depth--; depth == 0 {
					j--
					break
				}
			}
		}
		for j >= 0 && isSpaceByte(f.text[j]) {
			j--
		}
	}
	end := j + 1
	for j >= 0 && isTSIdentByte(f.text[j]) {
		j--
	}
	return f.text[j+1 : end]
}

// tsArrowName names an arrow function after what it is assigned to:
// "request" for "request: <T>(method: string) => ...".
func tsArrowName(f *srcFile, open int) string {
	j := open - 1
	for j >= 0 && isSpaceByte(f.text[j]) {
		j--
	}
	if j >= 0 && f.text[j] == '>' {
		for depth := 0; j >= 0; j-- {
			if f.text[j] == '>' {
				depth++
			} else if f.text[j] == '<' {
				if depth--; depth == 0 {
					j--
					break
				}
			}
		}
	}
	for j >= 0 && isSpaceByte(f.text[j]) {
		j--
	}
	if j >= 4 && f.text[j-4:j+1] == "async" {
		j -= 5
		for j >= 0 && isSpaceByte(f.text[j]) {
			j--
		}
	}
	if j < 0 || (f.text[j] != ':' && f.text[j] != '=') {
		return ""
	}
	j--
	for j >= 0 && isSpaceByte(f.text[j]) {
		j--
	}
	end := j + 1
	for j >= 0 && isTSIdentByte(f.text[j]) {
		j--
	}
	return f.text[j+1 : end]
}

// tsSkipReturnType returns the index of the { or => after a return type that
// starts at i, or -1 when what follows the colon is not a return type.
func tsSkipReturnType(f *srcFile, i int) int {
	angle := 0
	for j := i; j < len(f.text); j++ {
		if !f.code(j) {
			continue
		}
		switch c := f.text[j]; {
		case c == '<':
			angle++
		case c == '=' && j+1 < len(f.text) && f.text[j+1] == '>':
			if angle == 0 {
				return j
			}
			j++
		case c == '>':
			angle--
		case c == '(' || c == '[':
			j = f.match[j]
			if j < 0 {
				return -1
			}
		case c == '{':
			if angle == 0 {
				return j
			}
			if j = f.match[j]; j < 0 {
				return -1
			}
		case angle == 0 && strings.IndexByte(";,)]}", c) >= 0:
			return -1
		}
	}
	return -1
}

// tsExpressionEnd returns where an expression that starts at i ends.
func tsExpressionEnd(f *srcFile, i int) int {
	for j := i; j < len(f.text); j++ {
		if !f.code(j) {
			continue
		}
		switch c := f.text[j]; c {
		case '(', '[', '{':
			if m := f.match[j]; m > j {
				j = m
			}
		case ',', ';', ')', ']', '}':
			return j
		}
	}
	return len(f.text)
}

// tsCutDefault cuts a parameter's default value off its type.
func tsCutDefault(s string) string {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}':
			depth--
		case '>':
			if i == 0 || s[i-1] != '=' {
				depth--
			}
		case '=':
			if depth == 0 && (i+1 >= len(s) || s[i+1] != '>') {
				return s[:i]
			}
		}
	}
	return s
}

// enclosing returns the functions whose body holds off, innermost first.
func (src *tsSource) enclosing(off int) []*tsFunc {
	var out []*tsFunc
	for _, fn := range src.funcs {
		if fn.body.a <= off && off < fn.body.b {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].body.a > out[j].body.a })
	return out
}

func (src *tsSource) funcName(off int) string {
	for _, fn := range src.enclosing(off) {
		if fn.name != "" {
			return fn.name
		}
	}
	return "<module>"
}

// statementEnd returns the end of a declaration's value that starts at i.
func (src *tsSource) statementEnd(i int) int {
	for j := i; j < len(src.text); j++ {
		if !src.code(j) {
			continue
		}
		switch src.text[j] {
		case '(', '[', '{':
			if m := src.match[j]; m > j {
				j = m
			}
		case ';', ')', ']', '}':
			return j
		}
	}
	return len(src.text)
}

// tsDecl is what a name is bound to where it is used.
type tsDecl struct {
	src        *tsSource
	value      span // the initializer; empty for a parameter
	typ        string
	param      bool
	scopeStart int
}

// declaration resolves name at off: a const or let in an enclosing function,
// a parameter, a module-level declaration, or an import.
func (src *tsSource) declaration(name string, off int) (tsDecl, bool) {
	re := regexp.MustCompile(`\b(?:const|let|var)\s+(` + regexp.QuoteMeta(name) + `)\b`)
	for _, fn := range src.enclosing(off) {
		var last []int
		for _, m := range re.FindAllStringSubmatchIndex(src.text[fn.body.a:off], -1) {
			if src.code(fn.body.a + m[2]) {
				last = m
			}
		}
		if last != nil {
			return src.declAt(fn.body.a+last[3], fn.body.a)
		}
		if typ, ok := fn.params[name]; ok {
			return tsDecl{src: src, typ: typ, param: true, scopeStart: fn.body.a}, true
		}
	}
	return src.moduleDeclaration(name, 0)
}

// declAt reads the type and initializer that follow a declared name ending at i.
func (src *tsSource) declAt(i, scopeStart int) (tsDecl, bool) {
	j := tsSkipSpace(src.srcFile, i)
	typ := ""
	if j < len(src.text) && src.text[j] == ':' {
		k := j + 1
		for angle := 0; k < len(src.text); k++ {
			if !src.code(k) {
				continue
			}
			c := src.text[k]
			if c == '(' || c == '[' || c == '{' {
				k = src.match[k]
			} else if c == '<' {
				angle++
			} else if c == '>' && src.text[k-1] != '=' {
				angle--
			} else if c == '=' && angle == 0 && src.text[k+1] != '>' {
				break
			}
		}
		typ = strings.TrimSpace(src.text[j+1 : k])
		j = k
	}
	if j >= len(src.text) || src.text[j] != '=' {
		return tsDecl{src: src, typ: typ, scopeStart: scopeStart}, typ != ""
	}
	value := src.trim(span{j + 1, src.statementEnd(j + 1)})
	return tsDecl{src: src, value: value, typ: typ, scopeStart: scopeStart}, true
}

func (src *tsSource) moduleDeclaration(name string, depth int) (tsDecl, bool) {
	if depth > 4 {
		return tsDecl{}, false
	}
	re := regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(` + regexp.QuoteMeta(name) + `)\b`)
	for _, m := range src.codeMatches(re, 1) {
		if src.depth[m[2]] == 0 {
			return src.declAt(m[3], 0)
		}
	}
	if other, remote, ok := src.imported(name); ok {
		return other.moduleDeclaration(remote, depth+1)
	}
	return tsDecl{}, false
}

// imported finds the file a name is imported from, and its name there.
func (src *tsSource) imported(name string) (*tsSource, string, bool) {
	for _, m := range tsImportNames.FindAllStringSubmatch(src.text, -1) {
		for _, entry := range strings.Split(m[1], ",") {
			fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(entry), "type "))
			remote, local := "", ""
			switch len(fields) {
			case 1:
				remote, local = fields[0], fields[0]
			case 3:
				remote, local = fields[0], fields[2]
			}
			if local != name {
				continue
			}
			if other, ok := src.openRelative(m[2]); ok {
				return other, remote, true
			}
		}
	}
	return nil, "", false
}

func (src *tsSource) openRelative(spec string) (*tsSource, bool) {
	base := filepath.Join(filepath.Dir(src.abs), filepath.FromSlash(spec))
	for _, candidate := range []string{base + ".ts", filepath.Join(base, "index.ts")} {
		if other, err := src.prog.open(candidate); err == nil {
			return other, true
		}
	}
	return nil, false
}

// stringOf reads s as a string template; off is where names are looked up.
func (src *tsSource) stringOf(s span, off, depth int) (string, error) {
	if depth > 10 {
		return "", fmt.Errorf("the value is built too indirectly")
	}
	s = src.unwrap(s)
	if i := src.topLevelIndex(s, " as "); i > 0 {
		s = src.unwrap(span{s.a, i})
	}
	text := src.str(s)
	if branches := src.ternary(s); branches != nil {
		// A path chosen by a condition is read only when both branches
		// name the same path and differ in the query string.
		var paths []string
		var first string
		for _, branch := range branches {
			template, err := src.stringOf(branch, off, depth+1)
			if err != nil {
				return "", err
			}
			path, _, err := sdkPath(template)
			if err != nil {
				return "", err
			}
			paths = append(paths, path)
			if first == "" {
				first = template
			}
		}
		if paths[0] != paths[1] {
			return "", fmt.Errorf("the path depends on a condition: %s or %s", paths[0], paths[1])
		}
		return first, nil
	}
	switch {
	case text == "":
		return "", fmt.Errorf("an empty value")
	case (text[0] == '\'' || text[0] == '"') && tsQuoteEnd(src.text, s.a) == s.b:
		src.read[s.a] = true
		return text[1 : len(text)-1], nil
	case text[0] == '`' && tsTemplateEnd(src.text, s.a) == s.b:
		src.read[s.a] = true
		content := text[1 : len(text)-1]
		var b strings.Builder
		for j := 0; j < len(content); j++ {
			switch {
			case content[j] == '\\' && j+1 < len(content):
				b.WriteByte(content[j+1])
				j++
			case strings.HasPrefix(content[j:], "${"):
				end := tsInterpolationEnd(content, j+2)
				b.WriteString(src.field(content[j+2:end-1], off, depth))
				j = end - 1
			default:
				b.WriteByte(content[j])
			}
		}
		return b.String(), nil
	case tsIdentExpr.MatchString(text):
		decl, ok := src.declaration(text, off)
		switch {
		case ok && decl.param:
			return "{" + text + "}", nil
		case ok && decl.value.b > decl.value.a:
			return decl.src.stringOf(decl.value, decl.value.a, depth+1)
		}
		return "", fmt.Errorf("%s is not bound to a string here", text)
	}
	if parts := src.splitTop(s, '+'); len(parts) > 1 {
		var b strings.Builder
		for _, part := range parts {
			piece, err := src.stringOf(part, off, depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(piece)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("cannot read %q as a path", shortText(text))
}

// field reads one ${expression} of a template literal.
func (src *tsSource) field(expr string, off, depth int) string {
	expr = strings.TrimSpace(expr)
	if tsBaseExpr.MatchString(expr) {
		return sdkBaseURL
	}
	if tsIdentExpr.MatchString(expr) {
		if decl, ok := src.declaration(expr, off); ok && !decl.param && decl.value.b > decl.value.a {
			if s, err := decl.src.stringOf(decl.value, decl.value.a, depth+1); err == nil {
				return s
			}
		}
	}
	return placeholder(expr)
}

// membersOf reads the top-level members of the JSON body s holds.
func (src *tsSource) membersOf(s span, off, depth int) (map[string]bool, error) {
	if depth > 10 {
		return nil, fmt.Errorf("the body is built too indirectly")
	}
	s = src.unwrap(s)
	text := src.str(s)
	if strings.HasPrefix(text, "await ") {
		return src.membersOf(span{s.a + len("await "), s.b}, off, depth+1)
	}
	if strings.HasPrefix(text, "new URLSearchParams(") {
		// A form body: no JSON members; the call is checked for its path
		// and method.
		return map[string]bool{}, nil
	}
	for _, cast := range []string{" as ", " satisfies "} {
		if i := src.topLevelIndex(s, cast); i > 0 {
			return src.membersOf(span{s.a, i}, off, depth+1)
		}
	}
	if q := src.ternary(s); q != nil {
		out := map[string]bool{}
		for _, branch := range q {
			more, err := src.membersOf(branch, off, depth+1)
			if err != nil {
				return nil, err
			}
			merge(out, more)
		}
		return out, nil
	}
	out := map[string]bool{}
	switch {
	case text == "undefined" || text == "null":
		return out, nil
	case strings.HasPrefix(text, "{") && src.match[s.a] == s.b-1:
		for _, item := range src.items(s.a, false) {
			itemText := src.str(item)
			if strings.HasPrefix(itemText, "...") {
				more, err := src.membersOf(span{item.a + 3, item.b}, off, depth+1)
				if err != nil {
					return nil, err
				}
				merge(out, more)
				continue
			}
			k := tsObjectKey.FindStringSubmatch(itemText)
			if k == nil {
				return nil, fmt.Errorf("cannot read the object entry %q", shortText(itemText))
			}
			out[k[1]+k[2]] = true
		}
		return out, nil
	case tsStringCall.MatchString(text):
		open := s.a + strings.IndexByte(text, '(')
		if src.match[open] == s.b-1 {
			if args := src.items(open, false); len(args) > 0 {
				return src.membersOf(args[0], off, depth+1)
			}
		}
	case tsIdentExpr.MatchString(text):
		return src.nameMembers(text, off, depth)
	case tsCallExpr.MatchString(text):
		name := tsCallExpr.FindStringSubmatch(text)[1]
		open := s.a + strings.IndexByte(text, '(')
		if src.match[open] == s.b-1 {
			return src.returnedMembers(name, depth)
		}
	}
	return nil, fmt.Errorf("cannot read the body %q", shortText(text))
}

// ternary returns the two branches of a top-level conditional expression.
func (src *tsSource) ternary(s span) []span {
	for i := s.a; i < s.b; i++ {
		if !src.code(i) {
			continue
		}
		switch c := src.text[i]; c {
		case '(', '[', '{':
			if m := src.match[i]; m > i && m < s.b {
				i = m
			}
		case '?':
			if i+1 < s.b && (src.text[i+1] == '.' || src.text[i+1] == '?') || src.text[i-1] == '?' {
				continue
			}
			colon := src.topLevelIndex(span{i + 1, s.b}, ":")
			if colon < 0 {
				return nil
			}
			return []span{{i + 1, colon}, {colon + 1, s.b}}
		}
	}
	return nil
}

// nameMembers reads the members a variable holds at off: its initializer (or
// its declared type), with the members assigned on it before off.
func (src *tsSource) nameMembers(name string, off, depth int) (map[string]bool, error) {
	decl, ok := src.declaration(name, off)
	if !ok {
		return nil, fmt.Errorf("%s is not declared where it is sent", name)
	}
	out := map[string]bool{}
	var err error
	switch {
	case decl.value.b > decl.value.a:
		out, err = decl.src.membersOf(decl.value, decl.value.a, depth+1)
		if err != nil && decl.typ != "" {
			out, err = decl.src.typeMembers(decl.typ, depth+1)
		}
	case decl.typ != "":
		out, err = decl.src.typeMembers(decl.typ, depth+1)
	default:
		err = fmt.Errorf("%s has no initializer and no type", name)
	}
	if err != nil {
		return nil, err
	}
	if decl.src == src && decl.scopeStart < off {
		from := decl.scopeStart
		if decl.value.b > decl.value.a {
			from = decl.value.b
		}
		set := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `(?:\.([A-Za-z_$][\w$]*)|\[\s*['"]([^'"]+)['"]\s*\])\s*=(?:[^=]|$)`)
		for _, m := range set.FindAllStringSubmatchIndex(src.text[from:off], -1) {
			if !src.code(from + m[0]) {
				continue
			}
			if m[2] >= 0 {
				out[src.text[from+m[2]:from+m[3]]] = true
			} else {
				out[src.text[from+m[4]:from+m[5]]] = true
			}
		}
	}
	return out, nil
}

// returnedMembers reads the members of the object a function returns.
func (src *tsSource) returnedMembers(name string, depth int) (map[string]bool, error) {
	owner := src
	if other, remote, ok := src.imported(name); ok {
		owner, name = other, remote
	}
	re := regexp.MustCompile(`\bfunction\s+(` + regexp.QuoteMeta(name) + `)\s*[<(]`)
	for _, m := range owner.codeMatches(re, 1) {
		for _, fn := range owner.funcs {
			if fn.name != name || fn.body.a < m[0] || owner.text[fn.body.a] != '{' {
				continue
			}
			out := map[string]bool{}
			returns := 0
			ret := regexp.MustCompile(`\breturn\b`)
			for _, r := range ret.FindAllStringIndex(owner.text[fn.body.a:fn.body.b], -1) {
				at := fn.body.a + r[0]
				if !owner.code(at) || owner.depth[at] != owner.depth[fn.body.a]+1 {
					continue
				}
				returns++
				value := owner.trim(span{fn.body.a + r[1], owner.statementEnd(fn.body.a + r[1])})
				more, err := owner.membersOf(value, value.a, depth+1)
				if err != nil {
					return nil, err
				}
				merge(out, more)
			}
			if returns > 0 {
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot find what %s returns", name)
}

// typeMembers reads the property names of a TypeScript type.
func (src *tsSource) typeMembers(typ string, depth int) (map[string]bool, error) {
	if depth > 10 {
		return nil, fmt.Errorf("the type is built too indirectly")
	}
	typ = strings.TrimSpace(typ)
	var parts []string
	for _, part := range tsSplitType(typ, '|') {
		if part = strings.TrimSpace(part); part != "undefined" && part != "null" && part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) > 1 {
		out := map[string]bool{}
		for _, part := range parts {
			more, err := src.typeMembers(part, depth+1)
			if err != nil {
				return nil, err
			}
			merge(out, more)
		}
		return out, nil
	}
	if len(parts) == 1 {
		typ = parts[0]
	}
	if open := strings.IndexByte(typ, '<'); open > 0 && strings.HasSuffix(typ, ">") {
		generic, args := typ[:open], tsSplitType(typ[open+1:len(typ)-1], ',')
		switch generic {
		case "Partial", "Required", "Readonly":
			return src.typeMembers(args[0], depth+1)
		case "Omit", "Pick":
			if len(args) != 2 {
				break
			}
			keys := map[string]bool{}
			for _, k := range tsSplitType(args[1], '|') {
				keys[strings.Trim(strings.TrimSpace(k), `'"`)] = true
			}
			if generic == "Pick" {
				return keys, nil
			}
			out, err := src.typeMembers(args[0], depth+1)
			if err != nil {
				return nil, err
			}
			for k := range keys {
				delete(out, k)
			}
			return out, nil
		}
		return nil, fmt.Errorf("cannot read the members of %s", typ)
	}
	if strings.HasPrefix(typ, "{") && strings.HasSuffix(typ, "}") {
		return tsBodyMembers(typ[1 : len(typ)-1])
	}
	if !tsIdentExpr.MatchString(typ) {
		return nil, fmt.Errorf("cannot read the members of %s", typ)
	}
	owner, name := src, typ
	for hop := 0; hop < 5; hop++ {
		iface := regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(` + regexp.QuoteMeta(name) + `)\b([^{]*)\{`)
		for _, m := range owner.codeMatches(iface, 1) {
			open := m[1] - 1
			out, err := tsBodyMembers(owner.codeOnly(span{open + 1, owner.match[open]}))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if extends := strings.TrimSpace(owner.text[m[4]:m[5]]); strings.HasPrefix(extends, "extends ") {
				for _, base := range tsSplitType(strings.TrimPrefix(extends, "extends "), ',') {
					more, err := owner.typeMembers(base, depth+1)
					if err != nil {
						return nil, err
					}
					merge(out, more)
				}
			}
			return out, nil
		}
		alias := regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(` + regexp.QuoteMeta(name) + `)\b[^=]*=`)
		for _, m := range owner.codeMatches(alias, 1) {
			value := owner.trim(span{m[1], owner.statementEnd(m[1])})
			return owner.typeMembers(owner.codeOnly(value), depth+1)
		}
		other, remote, ok := owner.imported(name)
		if !ok {
			break
		}
		owner, name = other, remote
	}
	return nil, fmt.Errorf("cannot find the type %s", typ)
}

// codeOnly returns s's text with its comments blanked.
func (f *srcFile) codeOnly(s span) string {
	b := []byte(f.text[s.a:s.b])
	for i := range b {
		if f.class[s.a+i] == srcComment {
			b[i] = ' '
		}
	}
	return string(b)
}

// tsBodyMembers reads the property names of an interface or object type body.
func tsBodyMembers(body string) (map[string]bool, error) {
	out := map[string]bool{}
	var pieces []string
	for _, piece := range tsSplitType(body, ';') {
		for _, line := range tsSplitType(piece, '\n') {
			pieces = append(pieces, tsSplitType(line, ',')...)
		}
	}
	for _, piece := range pieces {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		if strings.HasPrefix(piece, "[") {
			return nil, fmt.Errorf("an index signature admits any member")
		}
		k := tsObjectKey.FindStringSubmatch(piece)
		if k == nil {
			continue
		}
		out[k[1]+k[2]] = true
	}
	return out, nil
}

// tsSplitType splits a type's text at top-level sep.
func tsSplitType(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'' || c == '"' || c == '`':
			for i++; i < len(s) && s[i] != c; i++ {
			}
		case c == '(' || c == '[' || c == '{' || c == '<':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '>' && (i == 0 || s[i-1] != '='):
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func extractTypeScriptCalls(t *testing.T, root string) sdkExtraction {
	t.Helper()
	prog := &tsProgram{root: root, files: map[string]*tsSource{}}
	var files []string
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "sdk", "typescript", "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, "Client.ts") || strings.HasSuffix(filepath.ToSlash(path), "/src/auth/oauth.ts")) {
			files = append(files, path)
		}
		return nil
	}))
	require.NotEmpty(t, files, "no *Client.ts under sdk/typescript/src")

	var ex sdkExtraction
	for _, abs := range files {
		src, err := prog.open(abs)
		require.NoError(t, err)
		for _, m := range src.codeMatches(tsSiteCall, 0) {
			ex.sites++
			helper := ""
			if m[2] >= 0 {
				helper = src.text[m[2]:m[3]]
			} else {
				helper = src.text[m[4]:m[5]]
			}
			line := src.line(m[0])
			function := src.funcName(m[0])
			fail := func(why string) {
				ex.unreadable = append(ex.unreadable, fmt.Sprintf("%s:%d (%s): %s", src.rel, line, function, why))
			}
			open := m[1] - 1
			if src.text[open] == '<' {
				for depth := 0; open < len(src.text); open++ {
					if src.text[open] == '<' {
						depth++
					} else if src.text[open] == '>' && src.text[open-1] != '=' {
						if depth--; depth == 0 {
							break
						}
					}
				}
				open = tsSkipSpace(src.srcFile, open+1)
			}
			if open >= len(src.text) || src.text[open] != '(' {
				fail("cannot find the call's arguments")
				continue
			}
			args := src.items(open, false)
			at := func(i int) (span, bool) {
				if i < len(args) {
					return args[i], true
				}
				return span{}, false
			}

			var methodArg, pathArg, bodyArg span
			var hasMethod, hasPath, hasBody bool
			method := ""
			switch helper {
			case "request", "requestAsAgent", "send":
				methodArg, hasMethod = at(0)
				pathArg, hasPath = at(1)
				bodyArg, hasBody = at(2)
			case "get", "delete":
				method, hasMethod = strings.ToUpper(helper), true
				pathArg, hasPath = at(0)
			case "post", "put", "patch":
				method, hasMethod = strings.ToUpper(helper), true
				pathArg, hasPath = at(0)
				bodyArg, hasBody = at(1)
			case "fetch":
				pathArg, hasPath = at(0)
				init, ok := at(1)
				if !ok || src.text[init.a] != '{' {
					fail("fetch without an options object")
					continue
				}
				for _, item := range src.items(init.a, false) {
					text := src.str(item)
					switch {
					case text == "method":
						methodArg, hasMethod = item, true
					case strings.HasPrefix(text, "method:"):
						methodArg, hasMethod = src.trim(span{item.a + len("method:"), item.b}), true
					case text == "body":
						bodyArg, hasBody = item, true
					case strings.HasPrefix(text, "body:"):
						bodyArg, hasBody = src.trim(span{item.a + len("body:"), item.b}), true
					}
				}
				if !hasMethod {
					method, hasMethod = "GET", true
				}
			}
			if !hasMethod || !hasPath {
				fail("the call names no method or no path")
				continue
			}

			forwardedMethod := false
			if method == "" {
				name := src.str(src.unwrap(methodArg))
				if decl, ok := src.declaration(name, methodArg.a); tsIdentExpr.MatchString(name) && ok && decl.param {
					forwardedMethod = true
				} else if s, err := src.stringOf(methodArg, methodArg.a, 0); err == nil {
					method = strings.ToUpper(s)
				} else {
					fail("cannot read the method " + shortText(name))
					continue
				}
			}

			template, err := src.stringOf(pathArg, pathArg.a, 0)
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
				if !tsRequestHelpers[function] {
					fail("passes on a path from its caller, but is not a request helper whose calls the test reads")
					continue
				}
				ex.forwarded = append(ex.forwarded, fmt.Sprintf("%s:%d (%s)", src.rel, line, function))
				continue
			}
			if forwardedMethod {
				fail("passes on a method from its caller for the fixed path " + path)
				continue
			}

			members := map[string]bool{}
			if hasBody {
				if members, err = src.membersOf(bodyArg, bodyArg.a, 0); err != nil {
					fail(err.Error())
					continue
				}
			}
			ex.calls = append(ex.calls, sdkCall{file: src.rel, line: line, function: function, method: method, path: path, members: sortedKeys(members)})
		}
	}
	for _, abs := range files {
		src, err := prog.open(abs)
		require.NoError(t, err)
		ex.unreadable = append(ex.unreadable, src.unreadServerPaths()...)
	}
	return ex
}
