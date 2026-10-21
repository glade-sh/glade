public class StringFormatQuotesProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<String> arguments = new List<String>{'Ada'};
        try { result.put('literal_quotes', String.format('value {0} equals \'Thing\'', arguments)); }
        catch(Exception issue) { result.put('literal_quotes', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('apostrophe', String.format('don\'t replace {0}', arguments)); }
        catch(Exception issue) { result.put('apostrophe', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('quoted_placeholder', String.format('value \'{0}\' then {0}', arguments)); }
        catch(Exception issue) { result.put('quoted_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('doubled_apostrophe', String.format('value \'\' then {0}', arguments)); }
        catch(Exception issue) { result.put('doubled_apostrophe', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('ids_parameter', String.format('The ids parameter should contain id\'s for A or B records, not {0} records.', new List<String>{'Account'})); }
        catch(Exception issue) { result.put('ids_parameter', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('label_single_apostrophe', String.format(Label.StringFormatSingle, new List<String>{'Account'})); }
        catch(Exception issue) { result.put('label_single_apostrophe', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('label_doubled_apostrophe', String.format(Label.StringFormatDoubled, new List<String>{'Account'})); }
        catch(Exception issue) { result.put('label_doubled_apostrophe', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('plain_placeholder', String.format('{0}', arguments)); }
        catch(Exception issue) { result.put('plain_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('unclosed_literal', String.format('value \'Thing then {0}', arguments)); }
        catch(Exception issue) { result.put('unclosed_literal', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('unclosed_placeholder', String.format('value \'{0} then {0}', arguments)); }
        catch(Exception issue) { result.put('unclosed_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('quoted_braces', String.format('\'{\'{0}\'}\'', arguments)); }
        catch(Exception issue) { result.put('quoted_braces', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('quoted_closing_brace', String.format('\'}\' {0}', arguments)); }
        catch(Exception issue) { result.put('quoted_closing_brace', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('doubled_around_placeholder', String.format('\'\'{0}\'\'', arguments)); }
        catch(Exception issue) { result.put('doubled_around_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('quoted_typed_placeholder', String.format('\'{0,number}\' {0}', arguments)); }
        catch(Exception issue) { result.put('quoted_typed_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('literal_then_placeholder', String.format('\'word\' {0}', arguments)); }
        catch(Exception issue) { result.put('literal_then_placeholder', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('adjacent_literals', String.format('\'one\' \'two\' {0}', arguments)); }
        catch(Exception issue) { result.put('adjacent_literals', issue.getTypeName()+':'+issue.getMessage()); }
        try { result.put('quote_before_comma', String.format('\'word,{0}\'', arguments)); }
        catch(Exception issue) { result.put('quote_before_comma', issue.getTypeName()+':'+issue.getMessage()); }
        return result;
    }
}
