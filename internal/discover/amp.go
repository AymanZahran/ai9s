package discover

// scanAmp lists no sessions. Amp keeps threads on ampcode.com.
// ai9s does not call the network to list them.
func scanAmp(_ func(string, int64) bool) Batch {
	return Batch{Agent: "amp"}
}
