//go:build race

package sema

// The race detector multiplies analysis time several times over, so the
// equality sweep runs fewer edits under it.
const incrementalRounds = 5
