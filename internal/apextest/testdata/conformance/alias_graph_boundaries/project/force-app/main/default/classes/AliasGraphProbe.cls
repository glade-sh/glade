public class AliasGraphProbe {
    public class Box {
        public Map<String,Object> items = new Map<String,Object>();
        public Box next;
        public Integer setterCalls = 0;
        public Integer getterCalls = 0;
        public Map<String,Object> exposed {
            get { getterCalls++; return items; }
            set { setterCalls++; items = value; }
        }
        public Map<String,Object> blocked {
            get { getterCalls++; return items; }
            set { setterCalls++; throw new IllegalArgumentException('blocked'); }
        }
        public void put(String key, Object item) { this.items.put(key, item); }
        public void putExposed(String key, Object item) { this.exposed.put(key, item); }
        public void putUnqualified(String key, Object item) { exposed.put(key,item); }
        public List<String> words = new List<String>();
        public List<String> wordView {get {return words;} set {setterCalls++; words=value;}}
        public Set<String> tags = new Set<String>();
        public Set<String> tagView {get {return tags;} set {setterCalls++; tags=value;}}
        public void putBlocked(String key, Object item) { this.blocked.put(key, item); }
    }
    public class Key {
        public Integer code;
        public Boolean fail = false;
        public Key(Integer value) { code=value; }
        public Integer hashCode() { if(fail) throw new IllegalArgumentException('key'); return code; }
        public Boolean equals(Object other) { return other instanceof Key && ((Key)other).code==code; }
    }
    public class KeyBox {
        public Map<Key,Object> items = new Map<Key,Object>();
        public void put(Key key,Object item) { this.items.put(key,item); }
    }
    public static Box root;
    public static Map<String,Box> registry;
    public static KeyBox keys;
    public static Integer staticSetterCalls = 0;
    public static Map<String,Object> staticItems = new Map<String,Object>();
    public static Map<String,Object> staticView {get {return staticItems;} set {staticSetterCalls++; staticItems=value;}}
    public static Map<String,Object> observe(String variant) {
        root=new Box(); registry=new Map<String,Box>{'root'=>root};
        Map<String,Object> out=new Map<String,Object>();
        if(variant=='direct') {
            List<String> words=new List<String>{'a'};
            root.put('words',words); root.put('warm',1); words.add('b');
            out.put('size',((List<String>)registry.get('root').items.get('words')).size());
            out.put('keys',root.items.size());
        } else if(variant=='knownChild') {
            Box child=new Box(); root.put('child',child); root.put('warm',1);
            List<String> words=new List<String>(); child.put('words',words);
            root.put('same',child); words.add('new');
            out.put('size',((List<String>)((Box)registry.get('root').items.get('child')).items.get('words')).size());
            out.put('same',root.items.get('same')===child);
        } else if(variant=='backReference') {
            root.put('warm',1); root.put('parent',root); root.put('last',2);
            Box linked=(Box)root.items.get('parent');
            out.put('identity',linked===root); out.put('keys',linked.items.size());
        } else if(variant=='cycle') {
            Box child=new Box(); child.next=root; root.next=child;
            root.put('warm',1); root.put('child',child); child.put('tail',new List<String>{'x'});
            List<String> words=(List<String>)child.items.get('tail'); words.add('y');
            out.put('identity',registry.get('root').next.next===root);
            out.put('size',((List<String>)((Box)root.items.get('child')).items.get('tail')).size());
        } else if(variant=='replacement') {
            root.put('warm',1); Map<String,Object> old=root.items;
            root.items=new Map<String,Object>(); root.put('fresh',new List<String>());
            ((List<String>)root.items.get('fresh')).add('x');
            out.put('oldSize',old.size()); out.put('oldHasFresh',old.containsKey('fresh'));
            out.put('newSize',registry.get('root').items.size()); out.put('size',((List<String>)root.items.get('fresh')).size());
        } else if(variant=='property') {
            root.put('warm',1); root.putExposed('fresh',new List<String>{'x'});
            out.put('setterCalls',root.setterCalls); out.put('keys',registry.get('root').items.size());
            out.put('size',((List<String>)root.items.get('fresh')).size());
        } else if(variant=='unqualifiedProperty') {
            root.putUnqualified('fresh',1);
            out.put('setterCalls',root.setterCalls); out.put('getterCalls',root.getterCalls); out.put('keys',root.items.size());
        } else if(variant=='externalProperty') {
            Box local=root; local.exposed.put('fresh',1);
            out.put('setterCalls',root.setterCalls); out.put('getterCalls',root.getterCalls); out.put('keys',root.items.size());
        } else if(variant=='nestedProperty') {
            root.next=new Box(); root.next.exposed.put('fresh',1);
            out.put('setterCalls',root.next.setterCalls); out.put('getterCalls',root.next.getterCalls); out.put('keys',root.next.items.size());
        } else if(variant=='staticProperty') {
            staticSetterCalls=0; staticItems=new Map<String,Object>();
            AliasGraphProbe.staticView.put('fresh',1);
            out.put('setterCalls',staticSetterCalls); out.put('keys',staticItems.size());
        } else if(variant=='listProperty') {
            root.wordView.add('fresh'); root.wordView.add('last');
            out.put('setterCalls',root.setterCalls); out.put('size',root.words.size());
        } else if(variant=='setProperty') {
            root.tagView.add('fresh'); root.tagView.add('last');
            out.put('setterCalls',root.setterCalls); out.put('size',root.tags.size());
        } else if(variant=='throwingProperty') {
            root.put('warm',1); String problem=null;
            try {root.putBlocked('fresh',new List<String>{'x'});} catch(Exception ex) {problem=ex.getTypeName();}
            root.put('last',2);
            out.put('exceptionType',problem); out.put('setterCalls',root.setterCalls);
            out.put('keys',registry.get('root').items.size()); out.put('hasFresh',root.items.containsKey('fresh'));
        } else if(variant=='setter') {
            root.put('warm',1); Map<String,Object> old=root.items;
            root.exposed=new Map<String,Object>(); root.put('fresh',1);
            out.put('setterCalls',root.setterCalls); out.put('keys',registry.get('root').items.size()); out.put('oldSize',old.size());
        } else if(variant=='keyFailure') {
            keys=new KeyBox(); Key good=new Key(1); keys.put(good,new List<String>{'a'});
            Key bad=new Key(2); bad.fail=true; String problem=null;
            try {keys.put(bad,root);} catch(Exception ex) {problem=ex.getTypeName();}
            keys.put(new Key(3),root); root.put('tail',1);
            out.put('exceptionType',problem); out.put('keys',keys.items.size());
            out.put('size',((List<String>)keys.items.get(new Key(1))).size());
            out.put('linked',((Box)keys.items.get(new Key(3))).items.containsKey('tail'));
        } else if(variant=='coercion') {
            List<Account> original=new List<Account>{new Account(Name='one')};
            List<SObject> widened=original; root.put('rows',widened); root.put('warm',1);
            widened.add(new Account(Name='two'));
            out.put('originalSize',original.size()); out.put('storedSize',((List<SObject>)registry.get('root').items.get('rows')).size());
            out.put('concrete',root.items.get('rows') instanceof List<Account>);
        }
        return out;
    }
}
