public class JsonAliasProbe {
    public static Integer constructors = 0;
    public static Publishing published;
    public class Child {
        public String Name = 'initializer';
        public Child() { constructors++; Name = 'constructor'; }
    }
    public class Envelope {
        public Child child;
        public Map<String, Child> children;
        public Envelope() {
            constructors++;
            child = new Child();
            children = new Map<String, Child>{'row' => child};
        }
    }
    public class Publishing {
        public String Name;
        public String Publish {
            get { return Name; }
            set { JsonAliasProbe.published = this; Name = value; }
        }
        public Publishing() { constructors++; JsonAliasProbe.published = this; }
    }
    public static Map<String, Object> observe(String shape, Boolean strict) {
        constructors = 0;
        published = null;
        Map<String, Object> cell = new Map<String, Object>{'shape' => shape, 'strict' => strict};
        if (shape == 'publication' || shape == 'setter' || shape == 'newInstance') {
            String body = shape == 'setter' ? '{"Publish":"initial"}' : '{}';
            Publishing root = shape == 'newInstance' ? (Publishing)Publishing.class.newInstance() :
                (Publishing)(strict ? JSON.deserializeStrict(body, Publishing.class) : JSON.deserialize(body, Publishing.class));
            cell.put('published', published != null);
            root.Name = 'after';
            cell.put('alias', published == null ? null : published.Name);
        } else if (shape == 'omitted') {
            Envelope root = (Envelope)(strict ? JSON.deserializeStrict('{}', Envelope.class) : JSON.deserialize('{}', Envelope.class));
            cell.put('fieldNull', root.child == null);
            cell.put('mapNull', root.children == null);
        } else {
            Child local = (Child)(strict ? JSON.deserializeStrict('{}', Child.class) : JSON.deserialize('{}', Child.class));
            cell.put('localName', local.Name);
            Child original;
            List<Child> listRoot;
            Set<Child> setRoot;
            Map<String, Child> mapRoot;
            Envelope fieldRoot;
            if (shape == 'list') {
                listRoot = (List<Child>)(strict ? JSON.deserializeStrict('[{}]', List<Child>.class) : JSON.deserialize('[{}]', List<Child>.class));
                original = listRoot[0];
            } else if (shape == 'set') {
                setRoot = (Set<Child>)(strict ? JSON.deserializeStrict('[{}]', Set<Child>.class) : JSON.deserialize('[{}]', Set<Child>.class));
                for (Child item : setRoot) { original = item; }
            } else if (shape == 'map') {
                mapRoot = (Map<String, Child>)(strict ? JSON.deserializeStrict('{"row":{}}', Map<String, Child>.class) : JSON.deserialize('{"row":{}}', Map<String, Child>.class));
                original = mapRoot.get('row');
            } else {
                fieldRoot = (Envelope)(strict ? JSON.deserializeStrict('{"child":{}}', Envelope.class) : JSON.deserialize('{"child":{}}', Envelope.class));
                original = fieldRoot.child;
            }
            cell.put('childName', original.Name);
            original.Name = 'after';
            if (shape == 'list') cell.put('alias', listRoot[0].Name);
            if (shape == 'set') for (Child item : setRoot) cell.put('alias', item.Name);
            if (shape == 'map') cell.put('alias', mapRoot.get('row').Name);
            if (shape == 'field') cell.put('alias', fieldRoot.child.Name);
            cell.put('localUnchanged', local.Name == null);
        }
        cell.put('constructors', constructors);
        return cell;
    }
    public static void capture() {
        List<Object> rows = new List<Object>();
        for (String shape : new List<String>{'list','set','map','field','omitted','publication','setter','newInstance'}) {
            for (Boolean strict : new List<Boolean>{false,true}) {
                rows.add(observe(shape, strict));
            }
        }
        System.assert(false, 'JSON_ALIAS|' + JSON.serialize(rows));
    }
}
