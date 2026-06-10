package com.glade.intellij.dap

import java.io.InputStream
import java.io.OutputStream
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class GladeDapProtocolTest {
    @Test
    fun gladeDapInitializesLaunchesAndTerminates() {
        val repoRoot = java.io.File("../..").canonicalFile
        val projectRoot = java.io.File(repoRoot, "internal/debuglog/testdata/project").canonicalPath
        val process = ProcessBuilder("go", "run", "./cmd/glade", "dap", "--project", projectRoot)
            .directory(repoRoot)
            .redirectError(ProcessBuilder.Redirect.PIPE)
            .start()

        try {
            val reader = DapReader(process.inputStream)
            process.outputStream.writeDap(1, "initialize", """{"clientID":"intellij-glade-test"}""")
            val initialize = reader.readMessage()
            assertTrue(initialize.contains(""""command":"initialize""""))
            assertTrue(initialize.contains(""""success":true"""))

            val initialized = reader.readMessage()
            assertTrue(initialized.contains(""""event":"initialized""""))

            process.outputStream.writeDap(2, "launch", """{"source":"Integer x = 1;\nSystem.debug('x=' + x);","project":"$projectRoot"}""")
            val launch = reader.readMessage()
            assertTrue(launch.contains(""""command":"launch""""))
            assertTrue(launch.contains(""""success":true"""))

            process.outputStream.writeDap(3, "disconnect", "{}")
            val disconnect = reader.readResponse("disconnect")
            assertTrue(disconnect.contains(""""command":"disconnect""""))
            assertTrue(disconnect.contains(""""success":true"""))
            assertTrue(process.waitFor(5, TimeUnit.SECONDS))
            assertEquals(0, process.exitValue())
        } finally {
            process.destroyForcibly()
        }
    }

    private fun OutputStream.writeDap(seq: Int, command: String, args: String) {
        val body = """{"seq":$seq,"type":"request","command":"$command","arguments":$args}"""
        val bytes = body.toByteArray(StandardCharsets.UTF_8)
        write("Content-Length: ${bytes.size}\r\n\r\n".toByteArray(StandardCharsets.UTF_8))
        write(bytes)
        flush()
    }

    private class DapReader(private val input: InputStream) {
        fun readMessage(): String {
            var length = -1
            while (true) {
                val line = readHeaderLine()
                if (line.isEmpty()) {
                    break
                }
                if (line.startsWith("Content-Length:", ignoreCase = true)) {
                    length = line.substringAfter(":").trim().toInt()
                }
            }
            require(length > 0) { "missing Content-Length" }
            return String(input.readNBytes(length), StandardCharsets.UTF_8)
        }

        private fun readHeaderLine(): String {
            val bytes = ArrayList<Byte>()
            while (true) {
                val b = input.read()
                if (b < 0) {
                    error("DAP stream closed before headers")
                }
                if (b == '\n'.code) {
                    break
                }
                if (b != '\r'.code) {
                    bytes.add(b.toByte())
                }
            }
            return bytes.toByteArray().toString(StandardCharsets.UTF_8)
        }

        fun readResponse(command: String): String {
            while (true) {
                val message = readMessage()
                if (message.contains(""""type":"response"""") && message.contains(""""command":"$command"""")) {
                    return message
                }
            }
        }
    }
}
