public class CovShapes {
    public interface Shape {
        Decimal area();
        String name();
    }

    public abstract class BaseShape implements Shape {
        public abstract Decimal area();

        public virtual String name() {
            return 'base';
        }

        public String describe() {
            return name() + ':' + area();
        }
    }

    public class Square extends BaseShape {
        private Decimal side;

        public Square(Decimal s) {
            side = s;
        }

        public override Decimal area() {
            return side * side;
        }

        public override String name() {
            return 'square';
        }
    }

    public static Decimal run(Decimal side) {
        Shape s = new Square(side);
        return s.area();
    }
}
