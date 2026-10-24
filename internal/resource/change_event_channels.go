package resource

import (
	"encoding/xml"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func applyChangeEventChannelMember(org *storage.OrgState, body []byte) error {
	var member struct {
		XMLName xml.Name `xml:"PlatformEventChannelMember"`
		Channel string   `xml:"eventChannel"`
		Entity  string   `xml:"selectedEntity"`
	}
	if err := xml.Unmarshal(body, &member); err != nil {
		return err
	}
	entity := strings.TrimSpace(member.Entity)
	if !strings.EqualFold(strings.TrimSpace(member.Channel), "ChangeEvents") || !strings.HasSuffix(strings.ToLower(entity), "changeevent") {
		return nil
	}
	for _, selected := range org.Metadata.ChangeEventEntities {
		if strings.EqualFold(selected, entity) {
			return nil
		}
	}
	org.Metadata.ChangeEventEntities = append(org.Metadata.ChangeEventEntities, entity)
	sort.Strings(org.Metadata.ChangeEventEntities)
	return nil
}
