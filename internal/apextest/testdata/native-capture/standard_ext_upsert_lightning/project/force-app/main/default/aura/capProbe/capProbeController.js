({ go: function(cmp) {
  var cases = window.location.hash ? decodeURIComponent(window.location.hash.substring(1)).split(';') : [];
  var lines = [];
  function next(i) {
    if (i >= cases.length) { cmp.set('v.out', 'DONE\n' + lines.join('\n')); return; }
    var p = cases[i].split(',');
    var a = p[1] === 'seed' ? cmp.get('c.seed') : cmp.get('c.run');
    a.setParams(p[1] === 'seed' ? {key: p[2]} : {kind: p[1], key: p[2]});
    a.setCallback(this, function(r) { lines.push(p[0] + '\t' + r.getState() + '\t' + (r.getState() === 'SUCCESS' ? r.getReturnValue() : JSON.stringify(r.getError()))); next(i + 1); });
    $A.enqueueAction(a);
  }
  next(0);
}})
