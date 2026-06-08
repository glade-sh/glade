(function () {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init)
  } else {
    init()
  }

  function init() {
    var root = document.querySelector(".VPLayout")
    if (!root) return
    initCopyButtons()
    initCommandPalette()
  }

  function initCopyButtons() {
    document.querySelectorAll("[data-copy-target]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var target = document.getElementById(btn.getAttribute("data-copy-target"))
        if (!target) return
        var text = target.textContent.trim()
        navigator.clipboard.writeText(text).then(function () {
          btn.textContent = "Copied"
          setTimeout(function () {
            btn.textContent = "Copy"
          }, 1400)
        })
      })
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
      '<a href="/guide/installation" class="home-cmd-item"><strong>Install</strong><code>curl -fsSL https://glade.sh/install.sh | sh</code></a>' +
      '<a href="/guide/cli-reference" class="home-cmd-item"><strong>Check source</strong><code>glade check --project . --json</code></a>' +
      '<a href="/guide/local-testing" class="home-cmd-item"><strong>Run tests</strong><code>glade test --project . --json</code></a>' +
      '<a href="/guide/installation" class="home-cmd-item"><strong>Open docs</strong><code>/guide/installation</code></a>' +
      '<a href="https://play.glade.sh/playground/" class="home-cmd-item"><strong>Open playground</strong><code>play.glade.sh/playground</code></a>' +
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
