// Fixture: scratch declarations the lint must NOT flag. These are sql query
// argument builders, JSON marshal/unmarshal scratch targets, and pq.Array
// Scan destinations whose downstream code either always overwrites the value
// or handles emptiness via len() before marshaling.
package fixture

func ScratchTargets() {
	var args []interface{}
	args = append(args, "value")
	_ = args

	var metadataJSON []byte
	_ = metadataJSON

	var objects []map[string]interface{}
	_ = objects

	var runes []rune
	_ = runes
}
