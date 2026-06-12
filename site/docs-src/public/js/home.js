(function () {
  var examples = {
    account: {
      source: [
        "public class RunMe {",
        "  public static void main() {",
        "    Account a = new Account(Name = 'Twin Lakes');",
        "    insert a;",
        "    System.debug([SELECT Name FROM Account].size());",
        "  }",
        "}"
      ].join("\n"),
      idle: {
        status: "Pass",
        timing: "38 ms",
        log: "USER_DEBUG | Account count: 1",
        state: "1 Account inserted · rolled back after run"
      },
      result: {
        status: "Pass",
        timing: "38 ms",
        log: "USER_DEBUG | Account count: 1",
        state: "1 Account inserted · rolled back after run"
      }
    },
    soql: {
      source: [
        "public class RunMe {",
        "  public static void main() {",
        "    List<Account> rows = [",
        "      SELECT Name FROM Account WHERE Name LIKE 'Twin%'",
        "    ];",
        "    System.debug(rows.size());",
        "  }",
        "}"
      ].join("\n"),
      idle: {
        status: "Not run",
        timing: "--",
        log: "Run Example to see output",
        state: "No query cursor yet"
      },
      result: {
        status: "Pass",
        timing: "24 ms",
        log: "USER_DEBUG | SOQL rows: 1",
        state: "local.sqlite · Account read"
      }
    },
    rollback: {
      source: [
        "public class RunMe {",
        "  public static void main() {",
        "    Savepoint sp = Database.setSavepoint();",
        "    insert new Account(Name = 'Temporary');",
        "    Database.rollback(sp);",
        "    System.debug([SELECT Id FROM Account].size());",
        "  }",
        "}"
      ].join("\n"),
      idle: {
        status: "Not run",
        timing: "--",
        log: "Run Example to see output",
        state: "No transaction opened"
      },
      result: {
        status: "Pass",
        timing: "42 ms",
        log: "USER_DEBUG | Records after rollback: 0",
        state: "local.sqlite · rollback restored state"
      }
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init)
  } else {
    init()
  }

  function init() {
    initHomeControls()
    initCommandPalette()
  }

  function initHomeControls() {
    document.addEventListener("click", function (e) {
      var target = e.target
      if (!target || !target.closest) return

      var copyButton = target.closest("[data-copy-target]")
      if (copyButton) {
        var copyTarget = document.getElementById(copyButton.getAttribute("data-copy-target"))
        if (!copyTarget) return
        var text = copyTarget.textContent.trim()
        navigator.clipboard.writeText(text).then(function () {
          copyButton.textContent = "Copied"
          setTimeout(function () {
            copyButton.textContent = "Copy"
          }, 1400)
        })
        return
      }

      var runButton = target.closest("[data-run-example]")
      if (runButton) {
        var activeId = activeExampleId()
        var example = examples[activeId]
        if (!example || runButton.hasAttribute("disabled")) return
        runButton.setAttribute("disabled", "true")
        setStatus("running")
        setTimeout(function () {
          setStatus("pass")
          setOutput(example.result)
          runButton.removeAttribute("disabled")
        }, 520)
        return
      }

      var exampleButton = target.closest("[data-example-id]")
      if (exampleButton) {
        setActiveExample(exampleButton.getAttribute("data-example-id"))
      }
    })
  }

  function activeExampleId() {
    var side = document.querySelector("[data-example-active]")
    return side ? side.getAttribute("data-example-active") : "account"
  }

  function setActiveExample(id) {
    var example = examples[id]
    var side = document.querySelector("[data-example-active]")
    var code = document.querySelector("[data-example-code]")
    if (!example || !side || !code) return

    side.setAttribute("data-example-active", id)
    document.querySelectorAll("[data-example-id]").forEach(function (button) {
      var active = button.getAttribute("data-example-id") === id
      button.classList.toggle("active", active)
      button.setAttribute("aria-pressed", active ? "true" : "false")
    })

    code.textContent = example.source
    if (window.gladeHighlightCodeBlock) {
      window.gladeHighlightCodeBlock(code)
    }
    setStatus(example.idle.status === "Pass" ? "pass" : "idle")
    setOutput(example.idle)
  }

  function setStatus(value) {
    var status = document.querySelector("[data-run-status]")
    if (!status) return
    status.textContent = value
    status.className = "home-status-pill home-status-" + value
  }

  function setOutput(output) {
    Object.keys(output).forEach(function (key) {
      var target = document.querySelector('[data-output-key="' + key + '"]')
      if (!target) return
      target.textContent = output[key]
      target.className = key === "status" && output[key] === "Pass" ? "home-output-pass" : ""
    })
  }

  function initCommandPalette() {
    var overlay = document.createElement("div")
    overlay.className = "home-cmd-overlay"
    overlay.setAttribute("aria-hidden", "true")
    overlay.innerHTML =
      '<div class="home-cmd-panel">' +
      '<div class="home-cmd-header"><span>glade</span><button class="home-cmd-close">esc</button></div>' +
      '<div class="home-cmd-items">' +
      '<a href="/guide/installation" class="home-cmd-item"><strong>Install Glade</strong><code>curl -fsSL https://glade.sh/install.sh | sh</code></a>' +
      '<a href="/guide/cli-reference" class="home-cmd-item"><strong>Check source</strong><code>glade check --project . --json</code></a>' +
      '<a href="/guide/local-testing" class="home-cmd-item"><strong>Run tests</strong><code>glade test --project . --json</code></a>' +
      '<a href="/guide/overview" class="home-cmd-item"><strong>Open docs</strong><code>/guide/overview</code></a>' +
      '<a href="/guide/playground" class="home-cmd-item"><strong>Playground Docs</strong><code>glade playground --examples --open</code></a>' +
      "</div></div>"
    document.body.appendChild(overlay)

    var close = function () {
      overlay.setAttribute("aria-hidden", "true")
    }
    var open = function () {
      overlay.setAttribute("aria-hidden", "false")
    }
    overlay.addEventListener("click", function (e) {
      if (e.target === overlay) close()
    })
    overlay.querySelector(".home-cmd-close").addEventListener("click", close)
    overlay.querySelectorAll(".home-cmd-item").forEach(function (item) {
      item.addEventListener("click", close)
    })
    window.addEventListener("keydown", function (e) {
      var tag = document.activeElement && document.activeElement.tagName
      var typing = tag === "INPUT" || tag === "TEXTAREA" || String(document.activeElement && document.activeElement.isContentEditable) === "true"
      if (!typing && (e.key === "/" || (e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey)))) {
        e.preventDefault()
        open()
      }
      if (e.key === "Escape") close()
    })
  }
})()
