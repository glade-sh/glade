@IsTest private class FlowTyped {
 static String p = '';
 static void add(String k, Object v) { p += '|' + k + '=' + v; }
 @IsTest static void t() {
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_numvar', new Map<String,Object>()); f.start(); add('Ft_numvar', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_numvar', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_numvar_text', new Map<String,Object>()); f.start(); add('Ft_numvar_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_numvar_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_numf', new Map<String,Object>()); f.start(); add('Ft_numf', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_numf', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_numf_text', new Map<String,Object>()); f.start(); add('Ft_numf_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_numf_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_boolvar', new Map<String,Object>()); f.start(); add('Ft_boolvar', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_boolvar', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_boolvar_text', new Map<String,Object>()); f.start(); add('Ft_boolvar_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_boolvar_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_boolf', new Map<String,Object>()); f.start(); add('Ft_boolf', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_boolf', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_boolf_text', new Map<String,Object>()); f.start(); add('Ft_boolf_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_boolf_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_datevar', new Map<String,Object>()); f.start(); add('Ft_datevar', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_datevar', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_datevar_text', new Map<String,Object>()); f.start(); add('Ft_datevar_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_datevar_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_datef', new Map<String,Object>()); f.start(); add('Ft_datef', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_datef', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_datef_text', new Map<String,Object>()); f.start(); add('Ft_datef_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_datef_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  try { Flow.Interview f = Flow.Interview.createInterview('Ft_strf_text', new Map<String,Object>()); f.start(); add('Ft_strf_text', 'ok:' + String.valueOf(f.getVariableValue('Out'))); } catch (Exception e) { add('Ft_strf_text', 'EX:' + e.getTypeName() + ':' + e.getMessage()); }
  System.assert(false, 'FlowTyped' + p);
 }
}
