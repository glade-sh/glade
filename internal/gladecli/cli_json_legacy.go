//go:build !goexperiment.jsonv2

package gladecli

import "io"

func writeParseCLIJSONEnvelope(w io.Writer, env cliJSONEnvelope) error {
	return writeCLIJSONEnvelope(w, env)
}
