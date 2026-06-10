package com.glade.intellij.model

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class GladeSourceTest {
    @Test
    fun selectionWinsOverDocumentText() {
        val source = GladeSource.fromText("Integer x = 1;\nSystem.debug(x);", 0, 14)
        assertEquals("Integer x = 1;", source)
    }

    @Test
    fun blankSourceReturnsNull() {
        assertNull(GladeSource.fromText("   \n\t", null, null))
    }

    @Test
    fun launchJsonEscapesAnonymousSource() {
        val json = GladeSource.launchJsonForSource(
            projectRoot = "/tmp/project",
            source = "System.debug('a \"quote\"');"
        )
        assertEquals(
            """
            {
              "type": "glade",
              "name": "Glade: Debug Anonymous Apex",
              "request": "launch",
              "project": "/tmp/project",
              "source": "System.debug('a \"quote\"');"
            }
            """.trimIndent(),
            json
        )
    }

    @Test
    fun launchJsonForProgramUsesProgramField() {
        val json = GladeSource.launchJsonForProgram("/tmp/project", "/tmp/project/classes/Foo.cls")
        assertEquals(
            """
            {
              "type": "glade",
              "name": "Glade Apex",
              "request": "launch",
              "project": "/tmp/project",
              "program": "/tmp/project/classes/Foo.cls"
            }
            """.trimIndent(),
            json
        )
    }
}
