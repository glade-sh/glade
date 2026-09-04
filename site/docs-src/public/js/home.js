(function () {
  var scenarioOrder = ["check", "test", "exec", "debug"]
  var outputViewOrder = ["output", "json", "trace"]
  var activeScenarioId = "check"
  var activeOutputView = "output"
  var runTimers = []
  var copyTimer = 0
  var copyControlsInitialized = false
  var homeControlsInitialized = false
  var workbenchHashListenerInitialized = false
  var scenarioAliases = {
    "check-source": "check",
    "test-changed": "test",
    "exec-apex": "exec",
    "debug-log": "debug",
    "logs": "debug"
  }

  var checkOutput = [
    "$ glade check --project . --no-progress",
    "Glade check",
    "",
    "✗ 1 diagnostic found",
    "",
    "force-app/main/default/classes/RefinementService.cls:2:3",
    'error GLADESEMA002 method "latestInvoice" references unknown type "Invoice__c"',
    "",
    "Try:",
    "  glade schema load --project .",
    "  glade check --project .",
    "",
    "Summary:",
    "  apex types     1",
    "  triggers       0",
    "  objects        0",
    "  diagnostics    1",
    "  exit code      1"
  ].join("\n")

  var checkJSON = [
    "{",
    '  "schemaVersion": "1.0",',
    '  "command": "check",',
    '  "status": "failed",',
    '  "exitCode": 1,',
    '  "project": {',
    '    "root": "/tmp/glade-home-account-field",',
    '    "sourceApiVersion": "60.0"',
    "  },",
    '  "summary": {',
    '    "types": 1,',
    '    "triggers": 0,',
    '    "objects": 0,',
    '    "diagnostics": 1',
    "  },",
    '  "diagnostics": [',
    "    {",
    '      "severity": "error",',
    '      "code": "GLADESEMA002",',
    '      "message": "method \\"latestInvoice\\" references unknown type \\"Invoice__c\\"",',
    '      "file": "force-app/main/default/classes/RefinementService.cls",',
    '      "line": 2,',
    '      "column": 3,',
    '      "why": "The Apex type reference is not present in local Apex, schema, or platform symbols.",',
    '      "try": ["glade schema load --project .", "glade check --project ."]',
    "    }",
    "  ],",
    '  "suggestions": ["glade schema load --project .", "glade check --project ."]',
    "}"
  ].join("\n")

  var checkTrace = [
    "$ glade check --project . --json --progress-json",
    '{"kind":"phase_start","phase":"check","label":"Loading project","at":"2026-06-13T02:11:48.373515Z"}',
    '{"kind":"phase_tick","phase":"check","label":"Loading metadata","current":1,"total":4,"at":"2026-06-13T02:11:48.373811Z"}',
    '{"kind":"phase_tick","phase":"check","label":"Indexing Apex symbols","current":2,"total":4,"at":"2026-06-13T02:11:48.373822Z"}',
    '{"kind":"phase_tick","phase":"check","label":"Running semantic checks","current":3,"total":4,"at":"2026-06-13T02:11:48.374006Z"}',
    '{"kind":"phase_end","phase":"check","label":"Semantic checks complete","current":4,"total":4,"at":"2026-06-13T02:11:50.294097Z"}',
    '{"kind":"done","label":"check complete","ok":false,"exitCode":1,"at":"2026-06-13T02:11:50.294125Z"}',
    "",
    checkJSON
  ].join("\n")

  var testOutput = [
    "$ glade test --project . --class PassingTest --no-progress",
    "Glade test",
    "",
    "1 selected, 1 passed, 0 failed",
    "",
    "Selected: 1",
    "Passed:   1",
    "Failed:   0",
    "Runtime:  1s",
    "",
    "  ✓  PassingTest.passes  8ms",
    "",
    "Next:",
    "  glade test --watch",
    "  glade test failed"
  ].join("\n")

  var testJSON = [
    "{",
    '  "schemaVersion": "1.0",',
    '  "command": "test",',
    '  "status": "passed",',
    '  "exitCode": 0,',
    '  "summary": {',
    '    "total": 1,',
    '    "passed": 1,',
    '    "failed": 0,',
    '    "skipped": 0,',
    '    "compileErrors": 0,',
    '    "runtimeErrors": 0,',
    '    "unsupported": 0,',
    '    "errors": 0,',
    '    "durationMs": 1662',
    "  },",
    '  "suggestions": ["glade test --watch", "glade test failed"],',
    '  "tests": [',
    "    {",
    '      "name": "PassingTest.passes",',
    '      "className": "PassingTest",',
    '      "methodName": "passes",',
    '      "status": "pass",',
    '      "durationMs": 11',
    "    }",
    "  ],",
    '  "data": {',
    '    "name": "glade test",',
    '    "suites": [',
    "    {",
    '      "name": "PassingTest",',
    '      "cases": [',
    "        {",
    '          "className": "PassingTest",',
    '          "methodName": "passes",',
    '          "status": "pass",',
    '          "durationMs": 11',
    "        }",
    "      ]",
    "    }",
    "    ]",
    "  }",
    "}"
  ].join("\n")

  var testTrace = [
    "$ glade test --project . --class PassingTest --json --progress-json",
    '{"kind":"phase_start","phase":"test","label":"Discovering tests","at":"2026-06-13T02:11:48.073214Z"}',
    '{"kind":"warn","phase":"test","label":"startup cache: fresh","at":"2026-06-13T02:11:48.186572Z"}',
    '{"kind":"phase_tick","phase":"test","label":"Running tests","total":1,"at":"2026-06-13T02:11:48.186601Z"}',
    '{"kind":"phase_tick","phase":"test","label":"compile test harness","total":1,"at":"2026-06-13T02:11:48.186642Z"}',
    '{"kind":"phase_tick","phase":"test","label":"compile complete 1480ms","total":1,"at":"2026-06-13T02:11:49.666883Z"}',
    '{"kind":"phase_tick","phase":"test","label":"running PassingTest.passes · 1 running · 1 left","total":1,"at":"2026-06-13T02:11:49.723848Z"}',
    '{"kind":"phase_tick","phase":"test","current":1,"total":1,"at":"2026-06-13T02:11:49.735651Z"}',
    '{"kind":"phase_tick","phase":"test","label":"tests complete","current":1,"total":1,"at":"2026-06-13T02:11:49.735696Z"}',
    '{"kind":"done","label":"1 passed, 0 failed, 0 errors · 2s","ok":true,"exitCode":0,"at":"2026-06-13T02:11:49.7357Z"}',
    "",
    testJSON
  ].join("\n")

  var execOutput = [
    "$ glade exec \"System.debug('local');\"",
    "Glade exec",
    "",
    "✓ Anonymous Apex executed",
    "",
    "Debug:",
    "  USER_DEBUG local",
    "",
    "Limits:",
    "  SOQL queries    0 / 100",
    "  DML statements  0 / 150",
    "  CPU time        1ms / 10000ms",
    "",
    "Log:",
    "  .glade/logs/exec-20260613T073242Z.log",
    "",
    "Next:",
    "  glade debug profile --log .glade/logs/exec-20260613T073242Z.log",
    "  glade db inspect"
  ].join("\n")

  var execJSON = [
    "{",
    '  "schemaVersion": "1.0",',
    '  "command": "exec",',
    '  "status": "passed",',
    '  "exitCode": 0,',
    '  "summary": {',
    '    "debugEvents": 1,',
    '    "soqlQueries": 0,',
    '    "dml": 0,',
    '    "cpuTimeMs": 1',
    "  },",
    '  "suggestions": ["glade debug profile --log <log>", "glade db inspect"],',
    '  "data": {',
    '    "debug": ["local"],',
    '    "limits": { "queries": 0, "dmlStatements": 0, "cpuTimeMs": 1 },',
    '    "limitMode": "permissive"',
    "  }",
    "}"
  ].join("\n")

  var execTrace = [
    "$ glade exec --trace /tmp/glade-home-exec-trace.json \"System.debug('local');\"",
    "Glade exec",
    "",
    "✓ Anonymous Apex executed",
    "",
    "Debug:",
    "  USER_DEBUG local",
    "",
    "Log:",
    "  .glade/logs/exec-20260613T073242Z.log",
    "",
    "$ cat /tmp/glade-home-exec-trace.json",
    "{",
    '  "format": "chrome-trace-event",',
    '  "version": 1,',
    '  "traceEvents": [',
    "    {",
    '      "name": "apex.statement.expr",',
    '      "cat": "apex.statement",',
    '      "ph": "i",',
    '      "ts": 0,',
    '      "pid": 1,',
    '      "tid": 1,',
    '      "s": "t",',
    '      "args": {',
    '        "column": 1,',
    '        "line": 1,',
    '        "op": "expr",',
    '        "sourceOffset": 0',
    "      }",
    "    },",
    "    {",
    '      "name": "apex.limits",',
    '      "cat": "apex.limits",',
    '      "ph": "i",',
    '      "ts": 1,',
    '      "pid": 1,',
    '      "tid": 1,',
    '      "s": "t",',
    '      "args": {',
    '        "asyncJobs": 0,',
    '        "batchJobs": 0,',
    '        "callouts": 0,',
    '        "cpuTimeMs": 1,',
    '        "dmlRows": 0,',
    '        "dmlStatements": 0,',
    '        "emailInvocations": 0,',
    '        "futureCalls": 0,',
    '        "heapSize": 0,',
    '        "queries": 0,',
    '        "queryRows": 0,',
    '        "queueableJobs": 0,',
    '        "runAs": 0,',
    '        "scheduledJobs": 0',
    "      }",
    "    }",
    "  ]",
    "}"
  ].join("\n")

  var debugOutput = [
    "$ glade debug profile --log logs/apex-debug.log",
    "Glade debug profile",
    "",
    "Events: 4",
    "",
    "Runtime:",
    "  SOQL queries    1 query / 1 rows",
    "  DML statements  1 statement / 1 rows",
    "  Callouts        0",
    "  CPU             0ms",
    "  Heap            0 bytes",
    "",
    "Hot events:",
    "  Rank  Event                  Count  Rows  Duration",
    "  1     apex.debug             2      0     0ms",
    "  2     SELECT Id, Name FRO... 1      1     0ms",
    "  3     apex.dml.insert        1      1     0ms",
    "",
    "Next:",
    "  glade debug explain --log logs/apex-debug.log --project ."
  ].join("\n")

  var debugJSON = [
    "$ glade debug profile --log logs/apex-debug.log --json",
    "{",
    '  "schemaVersion": "1.0",',
    '  "command": "debug profile",',
    '  "status": "passed",',
    '  "exitCode": 0,',
    '  "summary": {',
    '    "events": 4,',
    '    "limits": {',
    '      "soqlQueries": 1,',
    '      "soqlRows": 1,',
    '      "dml": 1,',
    '      "dmlRows": 1',
    "    }",
    "  },",
    '  "suggestions": ["glade debug explain --log logs/apex-debug.log --project ."],',
    '  "data": {',
    '    "events": 4,',
    '    "hot": [',
    '      { "name": "apex.debug", "count": 2 },',
    '      { "name": "SELECT Id, Name FROM Account WHERE Name = \\"Acme\\"", "rows": 1 },',
    '      { "name": "apex.dml.insert", "rows": 1 }',
    "    ]",
    "  }",
    "}"
  ].join("\n")

  var debugTrace = debugJSON

  var scenarios = {
    "check": {
      shortLabel: "Check",
      title: "Catch deploy issues",
      preview: "1 diagnostic caught",
      actionLabel: "Replay example",
      commandLabel: "glade check",
      command: "glade check --project . --no-progress",
      ciCommand: "glade check --project . --json --no-progress",
      sourceLabel: "RefinementService.cls:2",
      highlightedLine: 2,
      sourceCode: [
        "public with sharing class RefinementService {",
        "  public static Invoice__c latestInvoice() {",
        "    return null;",
        "  }",
        "}"
      ].join("\n"),
      outputViews: {
        output: checkOutput,
        json: checkJSON,
        trace: checkTrace
      },
      resultStatus: "failed",
      resultSummary: "FAILED · 1 diagnostic · 1 type checked · exit code 1",
      runningSummary: "RUNNING · loading metadata · indexing Apex · semantic checks",
      runningOutput: "$ glade check --project . --no-progress\n\nRunning locally...",
      resultMetrics: [
        ["Diagnostics", "1"],
        ["Types checked", "1"],
        ["Org calls", "0"],
        ["Exit code", "1"]
      ],
      runningMetrics: [
        ["Phase", "loading"],
        ["Types checked", "1"],
        ["Org calls", "0"],
        ["Exit code", "..."]
      ],
      proofTitle: "Local result",
      changedSummary: [
        "Deploy-blocking type reference caught locally.",
        "No Salesforce deploy required.",
        "No local records changed.",
        "JSON output available for CI."
      ],
      runningSummaryItems: [
        "Loading local metadata.",
        "Indexing Apex symbols.",
        "Running semantic checks.",
        "Preparing JSON diagnostics."
      ],
      supportStatus: "supported locally",
      installCommands: [
        "curl -fsSL https://glade.sh/install.sh | sh",
        "glade doctor",
        "glade check --project . --no-progress"
      ],
      installLabel: "Selected workflow: Catch deploy issues",
      docsHref: "/reference/cli"
    },
    "test": {
      shortLabel: "Test",
      title: "Run focused tests",
      preview: "1 passed · 0 failed",
      actionLabel: "Replay example",
      commandLabel: "glade test",
      command: "glade test --project . --class PassingTest --no-progress",
      ciCommand: "glade test --project . --class PassingTest --json --no-progress",
      sourceLabel: "PassingTest.cls",
      sourceCode: [
        "@IsTest",
        "private class PassingTest {",
        "  @IsTest",
        "  static void passes() {",
        "    System.assertEquals(1, 1);",
        "  }",
        "}"
      ].join("\n"),
      outputViews: {
        output: testOutput,
        json: testJSON,
        trace: testTrace
      },
      resultStatus: "passed",
      resultSummary: "PASSED · 1 test · 0 failed · exit code 0",
      runningSummary: "RUNNING · discovering tests · compiling harness · executing",
      runningOutput: "$ glade test --project . --class PassingTest --no-progress\n\nRunning locally...",
      resultMetrics: [
        ["Tests run", "1"],
        ["Failures", "0"],
        ["Runtime", "281ms"],
        ["Exit code", "0"]
      ],
      runningMetrics: [
        ["Phase", "running"],
        ["Tests run", "1"],
        ["Failures", "..."],
        ["Exit code", "..."]
      ],
      proofTitle: "Local result",
      changedSummary: [
        "PassingTest.passes ran in the local fixture.",
        "The test run reported 1 passed and 0 failed.",
        "The command returned exit code 0.",
        "No org deploy required."
      ],
      runningSummaryItems: [
        "Loading local metadata.",
        "Discovering test methods.",
        "Compiling the test harness.",
        "Running PassingTest.passes."
      ],
      supportStatus: "supported locally",
      installCommands: [
        "curl -fsSL https://glade.sh/install.sh | sh",
        "glade doctor",
        "glade test --project . --class PassingTest --no-progress"
      ],
      installLabel: "Selected workflow: Run focused tests",
      docsHref: "/guide/local-testing"
    },
    "exec": {
      shortLabel: "Exec",
      title: "Execute Apex locally",
      preview: "USER_DEBUG emitted",
      actionLabel: "Replay example",
      commandLabel: "glade exec",
      command: "glade exec \"System.debug('local');\"",
      ciCommand: "glade exec --json \"System.debug('local');\"",
      sourceLabel: "anonymous.apex",
      sourceCode: "System.debug('local');",
      outputViews: {
        output: execOutput,
        json: execJSON,
        trace: execTrace
      },
      resultStatus: "passed",
      resultSummary: "PASSED · USER_DEBUG emitted · exit code 0",
      runningSummary: "RUNNING · compiling snippet · executing Apex · collecting limits",
      runningOutput: "$ glade exec \"System.debug('local');\"\n\nRunning locally...",
      resultMetrics: [
        ["Debug lines", "1"],
        ["SOQL", "0"],
        ["DML", "0"],
        ["CPU", "1"]
      ],
      runningMetrics: [
        ["Phase", "execute"],
        ["Debug lines", "..."],
        ["SOQL", "..."],
        ["DML", "..."]
      ],
      proofTitle: "Local result",
      changedSummary: [
        "The debug log reports USER_DEBUG local.",
        "The limit block reports 0 SOQL and 0 DML.",
        "Trace output can be written for profiling.",
        "No live org was touched."
      ],
      runningSummaryItems: [
        "Loading local metadata.",
        "Compiling anonymous Apex.",
        "Collecting debug log events.",
        "Collecting limit counters."
      ],
      supportStatus: "supported locally",
      installCommands: [
        "curl -fsSL https://glade.sh/install.sh | sh",
        "glade doctor",
        "glade exec \"System.debug('local');\""
      ],
      installLabel: "Selected workflow: Execute Apex locally",
      docsHref: "/reference/cli"
    },
    "debug": {
      shortLabel: "Debug",
      title: "Profile debug logs",
      preview: "4 events parsed",
      actionLabel: "Replay example",
      commandLabel: "glade debug",
      command: "glade debug profile --log logs/apex-debug.log",
      ciCommand: "glade debug profile --log logs/apex-debug.log --json",
      sourceTitle: "Debug log input",
      sourceLabel: "subscriber.log:8",
      highlightedLine: 8,
      sourceCode: [
        "64.0 APEX_CODE,DEBUG;APEX_PROFILING,INFO;CALLOUT,INFO;DB,INFO;SYSTEM,DEBUG",
        "00:00:00.000 (0)|USER_INFO|[EXTERNAL]|005000000000000AAA|isv@example.com|GMT|GMT+00:00",
        "00:00:00.001 (1000000)|EXECUTION_STARTED",
        "00:00:00.002 (2000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run",
        "00:00:00.003 (3000000)|USER_DEBUG|[3]|INFO|start work",
        "00:00:00.004 (4000000)|DML_BEGIN|[5]|Op:Insert|Type:Account|Rows:1",
        "00:00:00.005 (5000000)|DML_END|[5]",
        "00:00:00.006 (6000000)|SOQL_EXECUTE_BEGIN|[6]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = 'Acme'",
        "00:00:00.007 (7000000)|SOQL_EXECUTE_END|[6]|Rows:1",
        "00:00:00.008 (8000000)|USER_DEBUG|[7]|DEBUG|rows=1",
        "00:00:00.009 (9000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run",
        "00:00:00.010 (10000000)|EXECUTION_FINISHED"
      ].join("\n"),
      outputViews: {
        output: debugOutput,
        json: debugJSON,
        trace: debugTrace
      },
      resultStatus: "passed",
      resultSummary: "PASSED · 4 events parsed · 1 log profile",
      runningSummary: "RUNNING · reading log · parsing events · profiling hotspots",
      runningOutput: "$ glade debug profile --log logs/apex-debug.log\n\nReading debug log...",
      resultMetrics: [
        ["Events", "4"],
        ["SOQL", "1"],
        ["DML", "1"],
        ["CPU", "0ms"]
      ],
      runningMetrics: [
        ["Phase", "profile"],
        ["Events", "..."],
        ["SOQL", "..."],
        ["DML", "..."]
      ],
      proofTitle: "Local result",
      changedSummary: [
        "The saved Salesforce log parsed locally.",
        "The profile found 4 categorized events.",
        "SOQL and DML counts came from the log.",
        "JSON output is available for trace tooling."
      ],
      runningSummaryItems: [
        "Reading debug log.",
        "Parsing log entries.",
        "Grouping SOQL, DML, and debug events.",
        "Preparing JSON profile output."
      ],
      runningSteps: [
        "Reading debug log...",
        "Reading debug log...\nParsing log entries..."
      ],
      supportStatus: "offline log analysis",
      installCommands: [
        "curl -fsSL https://glade.sh/install.sh | sh",
        "glade doctor",
        "glade debug profile --log apex.log"
      ],
      installLabel: "Selected workflow: Profile debug logs",
      docsHref: "/reference/cli#glade-debug"
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init)
  } else {
    init()
  }
  window.gladeInitHomeDemos = init
  window.addEventListener("glade:content-updated", init)

  function init() {
    initCopyControls()
    initWorkbenchDemo()
  }

  function initWorkbenchDemo() {
    if (!document.querySelector("[data-scenario-workbench]")) return
    initHomeControls()
    setScenarioFromHash()
    if (!workbenchHashListenerInitialized) {
      window.addEventListener("hashchange", setScenarioFromHash)
      workbenchHashListenerInitialized = true
    }
  }

  function initCopyControls() {
    if (copyControlsInitialized) return
    copyControlsInitialized = true
    document.addEventListener("click", function (e) {
      var target = e.target
      if (!target || !target.closest) return
      var copyButton = target.closest("[data-copy-target]")
      if (copyButton) copyToClipboard(copyButton)
    })
  }

  function initHomeControls() {
    if (homeControlsInitialized) return
    homeControlsInitialized = true
    document.addEventListener("click", function (e) {
      var target = e.target
      if (!target || !target.closest) return

      var copyButton = target.closest("[data-copy-target]")
      if (copyButton) {
        return
      }

      var demoLink = target.closest("[data-demo-link]")
      if (demoLink) {
        var workbench = document.querySelector("[data-scenario-workbench]")
        if (workbench) {
          e.preventDefault()
          workbench.scrollIntoView({ behavior: prefersReducedMotion() ? "auto" : "smooth", block: "start" })
          window.history.replaceState(null, "", "#local-apex-workbench")
          window.setTimeout(focusWorkbenchRunButton, prefersReducedMotion() ? 0 : 180)
        }
        return
      }

      var outputTab = target.closest("[data-output-tab]")
      if (outputTab) {
        e.preventDefault()
        setOutputView(outputTab.getAttribute("data-output-tab"))
        return
      }

      var runButton = target.closest("[data-run-scenario]")
      if (runButton) {
        e.preventDefault()
        runActiveScenario()
        return
      }

      var scenarioButton = target.closest("[data-scenario-id]")
      if (scenarioButton) {
        e.preventDefault()
        setActiveScenario(scenarioButton.getAttribute("data-scenario-id"))
      }
    })

    window.addEventListener("keydown", function (e) {
      if (!(e instanceof KeyboardEvent)) return
      var tag = document.activeElement && document.activeElement.tagName
      var typing = tag === "INPUT" || tag === "TEXTAREA" || String(document.activeElement && document.activeElement.isContentEditable) === "true"
      if (typing) return
      if (!isWorkbenchKeyboardScope()) return
      var focusedOutputTab = document.activeElement && document.activeElement.closest && document.activeElement.closest("[data-output-tab]")
      if (focusedOutputTab && (e.key === "ArrowRight" || e.key === "ArrowLeft" || e.key === "Home" || e.key === "End")) {
        e.preventDefault()
        if (e.key === "Home") {
          moveOutputTab(-activeOutputViewIndex())
          return
        }
        if (e.key === "End") {
          moveOutputTab(outputViewOrder.length - activeOutputViewIndex() - 1)
          return
        }
        moveOutputTab(e.key === "ArrowRight" ? 1 : -1)
        return
      }
      var focusedScenarioTab = document.activeElement && document.activeElement.closest && document.activeElement.closest("[data-scenario-id]")
      if (focusedScenarioTab && (e.key === "ArrowRight" || e.key === "ArrowLeft" || e.key === "Home" || e.key === "End")) {
        e.preventDefault()
        var currentIndex = scenarioOrder.indexOf(activeScenarioId)
        var nextIndex = e.key === "Home" ? 0 : e.key === "End" ? scenarioOrder.length - 1 : (currentIndex + (e.key === "ArrowRight" ? 1 : -1) + scenarioOrder.length) % scenarioOrder.length
        setActiveScenario(scenarioOrder[nextIndex])
        var nextTab = document.querySelector('[data-scenario-id="' + scenarioOrder[nextIndex] + '"]')
        if (nextTab) nextTab.focus()
      }
    })
  }

  function copyToClipboard(copyButton) {
    var copyTarget = document.getElementById(copyButton.getAttribute("data-copy-target"))
    if (!copyTarget) return
    var text = copyLinesFromTarget(copyTarget)
    var copyLabel = copyButton.getAttribute("data-copy-label") || copyButton.textContent
    copyButton.setAttribute("data-copy-label", copyLabel)
    copyText(text, function (success) {
      copyButton.textContent = success ? "Copied" : "Copy failed"
      setCopyStatus(success ? copyLabel + " copied to clipboard" : "Copy failed")
      window.setTimeout(function () {
        copyButton.textContent = copyLabel
      }, 1400)
    })
  }

  function copyLinesFromTarget(target) {
    return target.getAttribute("data-copy-text") || target.textContent.trim()
  }

  function copyText(text, done) {
    var settled = false

    function complete(success) {
      if (settled) return
      settled = true
      done(success)
    }

    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        navigator.clipboard.writeText(text).then(function () {
          complete(true)
        }, function () {
          complete(fallbackCopyText(text))
        })
        return
      } catch (e) {
        complete(fallbackCopyText(text))
        return
      }
    }

    complete(fallbackCopyText(text))
  }

  function fallbackCopyText(text) {
    var active = document.activeElement
    var input = document.createElement("textarea")
    input.value = text
    input.setAttribute("readonly", "readonly")
    input.style.position = "fixed"
    input.style.top = "-1000px"
    document.body.appendChild(input)
    input.select()
    var copied = false
    try {
      copied = document.execCommand("copy")
    } catch (e) {
      copied = false
    }
    document.body.removeChild(input)
    if (active && typeof active.focus === "function") active.focus({ preventScroll: true })
    return copied
  }

  function setCopyStatus(message) {
    var target = document.querySelector("[data-copy-status]")
    if (!target) return
    if (copyTimer) window.clearTimeout(copyTimer)
    target.textContent = message
    copyTimer = window.setTimeout(function () {
      target.textContent = ""
    }, 1600)
  }

  function isWorkbenchKeyboardScope() {
    var workbench = document.querySelector("[data-scenario-workbench]")
    var active = document.activeElement
    return !!(workbench && active && workbench.contains(active))
  }

  function focusWorkbenchRunButton() {
    var runButton = document.querySelector("[data-run-scenario]")
    if (runButton) runButton.focus({ preventScroll: true })
  }

  function prefersReducedMotion() {
    return !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches)
  }

  function setScenarioFromHash() {
    var hash = window.location.hash.replace(/^#/, "")
    var id = scenarios[hash] ? hash : scenarioAliases[hash]
    setActiveScenario(id || "check", false)
  }

  function setActiveScenario(id, updateHash) {
    var scenario = scenarios[id]
    if (!scenario) return
    activeScenarioId = id
    activeOutputView = "output"
    clearRunTimers()

    document.querySelectorAll("[data-scenario-id]").forEach(function (button) {
      var active = button.getAttribute("data-scenario-id") === id
      button.classList.toggle("active", active)
      button.setAttribute("data-active", active ? "true" : "false")
      button.setAttribute("aria-selected", active ? "true" : "false")
      button.tabIndex = active ? 0 : -1
      var label = button.querySelector("[data-selected-label]")
      if (label) label.textContent = active ? "Selected" : ""
    })

    setText("[data-command-label]", scenario.commandLabel)
    setText("[data-source-title]", scenario.sourceTitle || "Apex input")
    setText("[data-source-label]", scenario.sourceLabel)
    setText("[data-command-strip]", scenario.command)
    setText("[data-ci-command-strip]", scenario.ciCommand || scenario.command)
    setText("[data-support-status]", scenario.supportStatus)
    setText("[data-run-scenario]", scenario.actionLabel)
    setText("[data-result-summary]", scenario.resultSummary)
    setText("[data-proof-title]", scenario.proofTitle)

    var runButton = document.querySelector("[data-run-scenario]")
    if (runButton) {
      runButton.disabled = false
      runButton.setAttribute("data-run-state", "idle")
    }

    var count = scenarioOrder.indexOf(id) + 1
    setText("[data-workflow-count]", count + " / 4")
    var workflowCount = document.querySelector("[data-workflow-count]")
    if (workflowCount) workflowCount.setAttribute("aria-label", "Workflow " + count + " of 4")
    updateOutputTabs()
    renderOutputView(scenario)
    setStatus(scenario.resultStatus)
    renderMetrics(scenario.resultMetrics)
    renderChangedSummary(scenario.changedSummary)
    renderInstallCommands(scenario.installCommands)
    setText("[data-install-workflow-label]", scenario.installLabel)

    var docs = document.querySelector("[data-docs-link]")
    if (docs) docs.setAttribute("href", scenario.docsHref)

    var source = document.querySelector("[data-source-code]")
    if (source) {
      source.textContent = scenario.sourceCode
      if (scenario.highlightedLine) {
        source.setAttribute("data-highlighted-line", String(scenario.highlightedLine))
      } else {
        source.removeAttribute("data-highlighted-line")
      }
    }
    if (source && window.gladeHighlightCodeBlock) {
      window.gladeHighlightCodeBlock(source)
    }
    if (source) markHighlightedSourceLine(source, scenario.highlightedLine)

    if (updateHash !== false && window.location.hash !== "#" + id) {
      window.history.replaceState(null, "", "#" + id)
    }
  }

  function setOutputView(view) {
    var scenario = scenarios[activeScenarioId]
    if (!scenario || !scenario.outputViews[view]) return
    activeOutputView = view
    updateOutputTabs()
    renderOutputView(scenario)
  }

  function updateOutputTabs() {
    document.querySelectorAll("[data-output-tab]").forEach(function (button) {
      var active = button.getAttribute("data-output-tab") === activeOutputView
      button.classList.toggle("active", active)
      button.setAttribute("aria-selected", active ? "true" : "false")
      button.tabIndex = active ? 0 : -1
    })
  }

  function activeOutputViewIndex() {
    var index = outputViewOrder.indexOf(activeOutputView)
    return index < 0 ? 0 : index
  }

  function moveOutputTab(delta) {
    var nextIndex = (activeOutputViewIndex() + delta + outputViewOrder.length) % outputViewOrder.length
    setOutputView(outputViewOrder[nextIndex])
    var nextTab = document.querySelector('[data-output-tab="' + outputViewOrder[nextIndex] + '"]')
    if (nextTab) nextTab.focus({ preventScroll: true })
  }

  function renderOutputView(scenario) {
    var output = document.querySelector("[data-command-output]")
    if (!output) return
    output.innerHTML = highlightCommandOutput(scenario.outputViews[activeOutputView] || scenario.outputViews.output, activeOutputView)
    output.setAttribute("data-output-view", activeOutputView)
    var panel = document.getElementById("command-output-panel")
    if (panel) panel.setAttribute("aria-labelledby", "output-tab-" + activeOutputView)
    var workflowPanel = document.getElementById("workbench-demo-panel")
    if (workflowPanel) workflowPanel.setAttribute("aria-labelledby", activeScenarioId)
  }

  function runActiveScenario() {
    var scenario = scenarios[activeScenarioId]
    if (!scenario) return
    var runButton = document.querySelector("[data-run-scenario]")
    activeOutputView = "output"
    clearRunTimers()
    updateOutputTabs()
    setStatus("running")
    setText("[data-result-summary]", scenario.runningSummary)
    setText("[data-proof-title]", "Running locally")
    setCommandOutput(scenario.runningOutput, "output")
    renderMetrics(scenario.runningMetrics)
    renderChangedSummary(scenario.runningSummaryItems)
    if (runButton) {
      runButton.disabled = true
      runButton.textContent = "Replaying..."
      runButton.setAttribute("data-run-state", "running")
    }

    if (prefersReducedMotion()) {
      finishRun(scenario, runButton)
      return
    }

    var runSteps = scenario.runningSteps || [
      "Loading local metadata...",
      "Loading local metadata...\nIndexing Apex symbols..."
    ]

    runTimers.push(window.setTimeout(function () {
      setCommandOutput("$ " + scenario.command + "\n\n" + runSteps[0], "output")
    }, 260))

    runTimers.push(window.setTimeout(function () {
      setCommandOutput("$ " + scenario.command + "\n\n" + runSteps[1], "output")
    }, 560))

    runTimers.push(window.setTimeout(function () {
      finishRun(scenario, runButton)
    }, 940))
  }

  function finishRun(scenario, runButton) {
    setStatus(scenario.resultStatus)
    setText("[data-result-summary]", scenario.resultSummary)
    setText("[data-proof-title]", scenario.proofTitle)
    renderMetrics(scenario.resultMetrics)
    renderChangedSummary(scenario.changedSummary)
    renderOutputView(scenario)
    if (runButton) {
      runButton.disabled = false
      runButton.textContent = "Replay again"
      runButton.setAttribute("data-run-state", "complete")
    }
  }

  function clearRunTimers() {
    runTimers.forEach(function (timer) {
      window.clearTimeout(timer)
    })
    runTimers = []
  }

  function setText(selector, value) {
    var target = document.querySelector(selector)
    if (target) target.textContent = value
  }

  function setCommandOutput(value, view) {
    var target = document.querySelector("[data-command-output]")
    if (!target) return
    target.innerHTML = highlightCommandOutput(value, view || "output")
    target.setAttribute("data-output-view", view || "output")
  }

  function highlightCommandOutput(value, view) {
    var text = String(value || "")
    if (view === "json") return highlightJsonOutput(text)
    if (view === "trace") return highlightTraceOutput(text)
    return text.split("\n").map(highlightCliLine).join("\n")
  }

  function highlightCliLine(line) {
    if (line.indexOf("$ ") === 0) {
      return '<span class="cli-token cli-prompt">$</span> ' + highlightCliCommand(line.slice(2))
    }
    if (/^\s*(glade|curl)\b/.test(line)) return highlightCliCommand(line)
    if (/^\s*(error|failed|failure)\b/i.test(line) || line.indexOf("✗") >= 0) {
      return '<span class="cli-token cli-error">' + escapeHTML(line) + "</span>"
    }
    if (/^\s*(PASSED|FAILED|RUNNING)\b/.test(line)) {
      return '<span class="cli-token cli-status">' + escapeHTML(line) + "</span>"
    }
    if (/^[A-Za-z0-9_./-]+:\d+:\d+/.test(line)) {
      return '<span class="cli-token cli-path">' + escapeHTML(line) + "</span>"
    }
    if (/^\s*(Try|Summary):/.test(line)) {
      return '<span class="cli-token cli-section">' + escapeHTML(line) + "</span>"
    }
    return escapeHTML(line).replace(/\b\d+(?:ms)?\b/g, '<span class="cli-token cli-number">$&</span>')
  }

  function highlightCliCommand(command) {
    return escapeHTML(command)
      .replace(/\b(glade|curl)\b/g, '<span class="cli-token cli-command">$1</span>')
      .replace(/\b(check|test|exec|debug|profile|schema|load|doctor)\b/g, '<span class="cli-token cli-subcommand">$1</span>')
      .replace(/(--[A-Za-z0-9-]+)/g, '<span class="cli-token cli-flag">$1</span>')
  }

  function highlightJsonOutput(value) {
    var text = String(value || "")
    var html = ""
    var index = 0

    while (index < text.length) {
      var char = text[index]
      if (char === '"') {
        var end = index + 1
        var escaped = false
        while (end < text.length) {
          var next = text[end]
          if (next === '"' && !escaped) break
          escaped = next === "\\" && !escaped
          if (next !== "\\") escaped = false
          end += 1
        }
        var token = text.slice(index, Math.min(end + 1, text.length))
        var after = Math.min(end + 1, text.length)
        while (after < text.length && /\s/.test(text[after])) after += 1
        var className = text[after] === ":" ? "cli-json-key" : "cli-json-string"
        html += '<span class="cli-token ' + className + '">' + escapeHTML(token) + "</span>"
        index = Math.min(end + 1, text.length)
        continue
      }

      var numberMatch = text.slice(index).match(/^-?\d+(?:\.\d+)?/)
      if (numberMatch) {
        html += '<span class="cli-token cli-number">' + escapeHTML(numberMatch[0]) + "</span>"
        index += numberMatch[0].length
        continue
      }

      var literalMatch = text.slice(index).match(/^(true|false|null)\b/)
      if (literalMatch) {
        html += '<span class="cli-token cli-json-literal">' + literalMatch[1] + "</span>"
        index += literalMatch[1].length
        continue
      }

      html += escapeHTML(char)
      index += 1
    }

    return html
  }

  function highlightTraceOutput(value) {
    return String(value || "").split("\n").map(function (line) {
      if (line.indexOf("$ ") === 0) return highlightCliLine(line)
      if (/^\s*\{/.test(line)) return highlightJsonOutput(line)
      return escapeHTML(line)
        .replace(/\b(USER_DEBUG|DML_BEGIN|DML_END|SOQL_EXECUTE_BEGIN|SOQL_EXECUTE_END|EXECUTION_STARTED|EXECUTION_FINISHED|CODE_UNIT_STARTED|CODE_UNIT_FINISHED)\b/g, '<span class="cli-token cli-trace-event">$1</span>')
        .replace(/\b\d+(?:ms)?\b/g, '<span class="cli-token cli-number">$&</span>')
    }).join("\n")
  }

  function setStatus(status) {
    var summary = document.querySelector("[data-result-summary]")
    if (summary) summary.setAttribute("data-result-state", status)
    var target = document.querySelector("[data-result-status]")
    if (!target) return
    target.className = "home-status home-status-" + status
    target.innerHTML = '<span class="home-status-dot"></span>' + escapeHTML(status)
  }

  function renderMetrics(metrics) {
    var target = document.querySelector("[data-result-metrics]")
    if (!target) return
    target.textContent = ""
    metrics.forEach(function (metric) {
      var row = document.createElement("div")
      var label = document.createElement("dt")
      var value = document.createElement("dd")
      label.textContent = metric[0]
      value.textContent = metric[1]
      row.appendChild(label)
      row.appendChild(value)
      target.appendChild(row)
    })
  }

  function renderChangedSummary(changedSummary) {
    var target = document.querySelector("[data-changed-summary]")
    if (!target) return
    target.textContent = ""
    changedSummary.forEach(function (item) {
      var row = document.createElement("li")
      row.textContent = item
      target.appendChild(row)
    })
  }

  function renderInstallCommands(installCommands) {
    var target = document.querySelector("[data-install-commands]")
    if (!target) return
    target.textContent = installCommands.join("\n")
  }

  function pluralize(count, singular, plural) {
    return count + " " + (count === 1 ? singular : (plural || singular + "s"))
  }

  function markHighlightedSourceLine(source, highlightedLine) {
    source.querySelectorAll(".line").forEach(function (line, index) {
      if (highlightedLine && index + 1 === highlightedLine) {
        line.setAttribute("data-highlighted", "true")
      } else {
        line.removeAttribute("data-highlighted")
      }
    })
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, function (char) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      }[char]
    })
  }
})()
