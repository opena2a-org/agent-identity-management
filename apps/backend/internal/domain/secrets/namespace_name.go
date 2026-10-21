package secrets

import "strings"

// A resolve request signs namespace|operation|nonce, so the signed text starts
// with the namespace. The TypeScript and Python SDKs refuse to sign a
// namespace that holds a line feed, a carriage return or a vertical bar, which
// would make the text read as other fields, or that begins with "DSSEv1 ", as
// a DSSE statement signed with an agent key begins. NamespaceNameRefusal
// states the refusal of the same names, so a namespace is not created under a
// name those SDKs then refuse to resolve.

// namespaceNameCharacters are the characters a namespace name must not hold,
// each with the words its refusal names it by.
var namespaceNameCharacters = []struct{ character, named string }{
	{"\n", "a line feed"},
	{"\r", "a carriage return"},
	{"|", "a vertical bar (|)"},
}

// namespaceNameDSSEPrefix is the start of the DSSE v1 pre-authentication
// encoding of a statement.
const namespaceNameDSSEPrefix = "DSSEv1 "

// NamespaceNameRefusal returns the line that names the rule name breaks, or ""
// when those SDKs' resolve signs it.
func NamespaceNameRefusal(name string) string {
	for _, c := range namespaceNameCharacters {
		if strings.Contains(name, c.character) {
			return "namespace must not contain " + c.named + ": resolve signs namespace|operation|nonce"
		}
	}
	if strings.HasPrefix(name, namespaceNameDSSEPrefix) {
		return `namespace must not begin with "DSSEv1 ", the first bytes of a DSSE statement an agent key signs`
	}
	return ""
}
