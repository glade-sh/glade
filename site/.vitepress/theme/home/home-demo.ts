/** Fixed illustrative source only. Nothing is compiled, executed or sent anywhere. */
export const INSTALL_COMMAND = 'curl -fsSL https://glade.sh/install.sh | sh'
export const commands = {
 tests: 'glade test --project . --class AccountServiceTest',
 debug: 'glade dap --project . --db .glade/envs/dev.sqlite',
 check: 'glade check --project .'
}
export function tokenize(line: string): { text: string; kind: string }[] {
 const pattern = /(\/\/.*$|'(?:[^'\\]|\\.)*'|@IsTest|\b(?:public|private|static|class|void|return|new|insert|SELECT|FROM|WHERE)\b|\b(?:AccountServiceTest|AccountService|Account|String|List|Assert)\b|\b\d+\b|\b[A-Za-z_][A-Za-z_0-9]*(?=\())/g
 const tokens: { text: string; kind: string }[] = []
 let last = 0
 for (const match of line.matchAll(pattern)) {
  const at = match.index ?? 0
  if (at > last) tokens.push({ text: line.slice(last, at), kind: '' })
  const text = match[0]
  const kind = text.startsWith('//') ? 't-comment' : text.startsWith("'") ? 't-string' : /^\d+$/.test(text) ? 't-num' : /^(AccountServiceTest|AccountService|Account|String|List|Assert)$/.test(text) ? 't-type' : /^(public|private|static|class|void|return|new|insert|SELECT|FROM|WHERE|@IsTest)$/.test(text) ? 't-key' : 't-fn'
  tokens.push({ text, kind }); last = at + text.length
 }
 if (last < line.length) tokens.push({ text: line.slice(last), kind: '' })
 return tokens
}
