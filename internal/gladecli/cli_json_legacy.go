//go:build !goexperiment.jsonv2

package gladecli

import (
	"encoding/json"
	"io"
)

func writeParseCLIJSONEnvelope(w io.Writer, env cliJSONEnvelope) error {
	env.SchemaVersion = cliJSONSchemaVersion
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
