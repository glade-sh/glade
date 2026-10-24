Map<String,Object> a = new Map<String,Object>{'label'=>'a'};
Map<String,Object> b = new Map<String,Object>{'label'=>'b'};
a.put('next',b); b.put('next',a);
Map<String,Object> self = new Map<String,Object>(); self.put('self',self);
Map<String,Object> wrapped = new Map<String,Object>{'outer'=>new List<Object>{self}};
List<Object> listRoot = new List<Object>{self};
for (String shape : new List<String>{'labelledPair','wrapped','listRoot'}) {
    Object graph = shape == 'labelledPair' ? a : (shape == 'wrapped' ? (Object)wrapped : (Object)listRoot);
    for (String operation : new List<String>{'serialize','generator'}) {
        Map<String,Object> row = new Map<String,Object>();
        try {
            String value;
            if (operation == 'serialize') value = JSON.serialize(graph);
            else { System.JSONGenerator g=JSON.createGenerator(false); g.writeObject(graph); value=g.getAsString(); }
            row.put('value',value);
        } catch (Exception e) { row.put('type',e.getTypeName()); row.put('message',e.getMessage()); }
        System.debug('JSON_CYCLE|'+shape+'|'+operation+'|'+JSON.serialize(row));
    }
}
