public class LayoutPresenceProbe {
    public static Map<String, Object> observe() {
        Metadata.Layout empty = new Metadata.Layout();
        Map<String, Object> observation = new Map<String, Object>{
            'empty_sections_null' => empty.layoutSections == null
        };
        try {
            List<Metadata.Metadata> layouts = Metadata.Operations.retrieve(
                Metadata.MetadataType.Layout,
                new List<String>{'Account-MeadowMissingLayout'}
            );
            observation.put('missing_layout_count', layouts.size());
        } catch (Exception problem) {
            observation.put('missing_layout_error', problem.getTypeName());
        }
        try {
            List<Metadata.Metadata> declared = Metadata.Operations.retrieve(
                Metadata.MetadataType.Layout,
                new List<String>{'Account-Meadow Layout'}
            );
            observation.put('declared_layout_count', declared.size());
            if (!declared.isEmpty()) {
                Metadata.Layout layout = (Metadata.Layout)declared[0];
                observation.put('declared_layout_full_name', layout.fullName);
                observation.put('declared_section_count', layout.layoutSections.size());
                if (!layout.layoutSections.isEmpty()) {
                    Metadata.LayoutSection section = layout.layoutSections[0];
                    observation.put('declared_section_label', section.label);
                    observation.put('declared_section_style', String.valueOf(section.style));
                    List<String> fields = new List<String>();
                    for (Metadata.LayoutColumn column : section.layoutColumns) {
                        for (Metadata.LayoutItem item : column.layoutItems) {
                            fields.add(item.field);
                        }
                    }
                    observation.put('declared_fields', fields);
                }
            }
        } catch (Exception problem) {
            observation.put('declared_layout_error', problem.getTypeName());
        }
        return observation;
    }
}
