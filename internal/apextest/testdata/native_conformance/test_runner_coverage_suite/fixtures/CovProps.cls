public class CovProps {
    public String label { get; set; }

    private Integer hitCount;

    public Integer Count {
        get {
            return hitCount == null ? 0 : hitCount;
        }
        set {
            hitCount = value;
        }
    }

    public String Upper {
        get {
            return label == null ? '' : label.toUpperCase();
        }
    }

    public String NeverSet {
        get {
            return 'fixed';
        }
        set {
            label = value;
        }
    }

    public static CovProps build(String l) {
        CovProps p = new CovProps();
        p.label = l;
        p.Count = 3;
        return p;
    }
}
