package com.glade.intellij.model

object GladeSource {
    fun fromText(text: String, selectionStart: Int?, selectionEnd: Int?): String? {
        val selected = if (selectionStart != null && selectionEnd != null && selectionEnd > selectionStart) {
            text.substring(selectionStart, selectionEnd)
        } else {
            text
        }
        return selected.trim().ifEmpty { null }
    }

    fun launchJsonForSource(projectRoot: String, source: String): String {
        return """
            {
              "type": "glade",
              "name": "Glade: Debug Anonymous Apex",
              "request": "launch",
              "project": "${json(projectRoot)}",
              "source": "${json(source)}"
            }
            """.trimIndent()
    }

    fun launchJsonForProgram(projectRoot: String, program: String): String {
        return """
            {
              "type": "glade",
              "name": "Glade Apex",
              "request": "launch",
              "project": "${json(projectRoot)}",
              "program": "${json(program)}"
            }
            """.trimIndent()
    }

    private fun json(value: String): String {
        val out = StringBuilder(value.length + 8)
        for (ch in value) {
            when (ch) {
                '\\' -> out.append("\\\\")
                '"' -> out.append("\\\"")
                '\n' -> out.append("\\n")
                '\r' -> out.append("\\r")
                '\t' -> out.append("\\t")
                else -> out.append(ch)
            }
        }
        return out.toString()
    }
}
