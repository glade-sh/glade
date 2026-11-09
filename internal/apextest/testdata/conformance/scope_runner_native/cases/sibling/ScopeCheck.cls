@IsTest private class ScopeCheck { @IsTest static void check() { Map<String,Object> value = new Map<String,Object>{'n' => 1}; ScopeProbe.run(new List<Map<String,Object>>{value, value}); } }
