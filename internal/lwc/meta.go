package lwc

import (
	"encoding/xml"
	"os"
)

type ComponentMeta struct {
	IsExposed bool     `xml:"isExposed"`
	Targets   []string `xml:"targets>target"`
}

func ParseComponentMeta(path string) (ComponentMeta, error) {
	if path == "" {
		return ComponentMeta{}, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ComponentMeta{}, err
	}
	var meta ComponentMeta
	if err := xml.Unmarshal(data, &meta); err != nil {
		return ComponentMeta{}, err
	}
	return meta, nil
}
