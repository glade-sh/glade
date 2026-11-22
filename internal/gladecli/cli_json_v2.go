//go:build goexperiment.jsonv2

package gladecli

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
)

func marshalCLIJSONEnvelope(env cliJSONEnvelope) ([]byte, error) {
	data, err := jsonv2.Marshal(env, json.DefaultOptionsV1(), jsontext.WithIndent("  "))
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeParseCLIJSONEnvelope(w io.Writer, env cliJSONEnvelope) error {
	env.SchemaVersion = cliJSONSchemaVersion
	data, err := marshalCLIJSONEnvelope(env)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
