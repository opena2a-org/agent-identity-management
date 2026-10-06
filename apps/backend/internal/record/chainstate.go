package record

// ChainState is the state of one scope's chain. Every surface that reports
// it, and the writer that refuses to extend a chain, use these three values
// and no others.
type ChainState string

const (
	// ChainNotStarted: the scope has no genesis record.
	ChainNotStarted ChainState = "notStarted"
	// ChainExtendable: the chain's stored head names its newest record, and
	// that record's stored bytes hash to the head's hash, so the next record
	// can link to it.
	ChainExtendable ChainState = "extendable"
	// ChainNotExtendable: the chain started, but its head cannot be linked
	// to: its genesis is missing, the stored head does not name the newest
	// record, or the newest record's bytes do not hash to its stored hash.
	ChainNotExtendable ChainState = "notExtendable"
)
