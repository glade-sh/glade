public with sharing class IPv4BoundaryProbe {
    public class AddressFormatException extends Exception {}
    public interface PackedAddress { Long numberValue(); }
    private class OctetAddress implements PackedAddress {
        private final Long value;
        private OctetAddress(String text) {
            List<String> pieces = text.split('\\.');
            if (pieces.size() != 4) { throw new AddressFormatException('octet count: ' + text); }
            final Long north;
            final Long east;
            final Long south;
            final Long west;
            try {
                north = Long.valueOf(pieces[0]);
                east = Long.valueOf(pieces[1]);
                south = Long.valueOf(pieces[2]);
                west = Long.valueOf(pieces[3]);
            } catch (Exception error) {
                throw new AddressFormatException(error.getTypeName() + ': ' + error.getMessage());
            }
            for (Long octet : new List<Long>{north, east, south, west}) {
                if (octet < 0 || octet > 255) { throw new AddressFormatException('octet range: ' + text); }
            }
            value = ((north * 256 + east) * 256 + south) * 256 + west;
        }
        public Long numberValue() { return value; }
    }
    public static PackedAddress create(String text) { return new OctetAddress(text); }
    public static Long parse(String text) {
        List<String> pieces = text.split('\\.');
        if (pieces.size() != 4) { throw new AddressFormatException('octet count: ' + text); }
        final Long north;
        final Long east;
        final Long south;
        final Long west;
        try {
            north = Long.valueOf(pieces[0]);
            east = Long.valueOf(pieces[1]);
            south = Long.valueOf(pieces[2]);
            west = Long.valueOf(pieces[3]);
        } catch (Exception error) {
            throw new AddressFormatException(error.getTypeName() + ': ' + error.getMessage());
        }
        for (Long octet : new List<Long>{north, east, south, west}) {
            if (octet < 0 || octet > 255) { throw new AddressFormatException('octet range: ' + text); }
        }
        return ((north * 256 + east) * 256 + south) * 256 + west;
    }
    public static void validateAddresses() {
        System.assertEquals(0L, create('0.0.0.0').numberValue());
        System.assertEquals(4294967295L, create('255.255.255.255').numberValue());
        System.assertEquals(520093951L, parse('31.0.0.255'));
        for (String north : new List<String>{'0', '31', '255'}) {
            for (String east : new List<String>{'0', '31', '255'}) {
                for (String south : new List<String>{'0', '31', '255'}) {
                    for (String west : new List<String>{'0', '31', '255'}) {
                        String text = String.join(new List<String>{north, east, south, west}, '.');
                        Long expected = ((Long.valueOf(north) * 256 + Long.valueOf(east)) * 256 + Long.valueOf(south)) * 256 + Long.valueOf(west);
                        System.assertEquals(expected, parse(text), 'direct ' + text);
                        System.assertEquals(expected, create(text).numberValue(), 'constructor ' + text);
                    }
                }
            }
        }
    }
}
