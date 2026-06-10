package com.glade.intellij.actions

import com.glade.intellij.model.GladeSource
import com.glade.intellij.util.GladePaths
import com.intellij.execution.ExecutionManager
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.executors.DefaultRunExecutor
import com.intellij.execution.process.OSProcessHandler
import com.intellij.execution.process.ProcessTerminatedListener
import com.intellij.execution.ui.ConsoleView
import com.intellij.execution.ui.ConsoleViewContentType
import com.intellij.execution.ui.RunContentDescriptor
import com.intellij.execution.ui.RunContentManager
import com.intellij.execution.impl.ConsoleViewImpl
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.actionSystem.PlatformDataKeys
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages

class ExecuteAnonymousAction : AnAction() {
    override fun update(event: AnActionEvent) {
        val file = event.getData(CommonDataKeys.VIRTUAL_FILE)
        event.presentation.isEnabledAndVisible = GladePaths.isApexFile(file)
    }

    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        val source = sourceFromEvent(event) ?: return
        runGladeExec(project, source)
    }

    private fun sourceFromEvent(event: AnActionEvent): String? {
        val editor = event.getData(PlatformDataKeys.EDITOR)
        if (editor != null) {
            val documentText = editor.document.text
            val selectionModel = editor.selectionModel
            val start = if (selectionModel.hasSelection()) selectionModel.selectionStart else null
            val end = if (selectionModel.hasSelection()) selectionModel.selectionEnd else null
            val source = GladeSource.fromText(documentText, start, end)
            if (source != null) {
                return source
            }
        }
        val entered = Messages.showMultilineInputDialog(
            event.project,
            "Enter anonymous Apex to execute with glade.",
            "Glade: Execute Anonymous Apex",
            "",
            null,
            null
        )
        return entered?.trim()?.ifEmpty { null }
    }

    private fun runGladeExec(project: Project, source: String) {
        val projectRoot = GladePaths.projectRoot(project)
        val commandLine = GeneralCommandLine("glade", "exec", "--debug-log", "-", source)
            .withWorkDirectory(projectRoot)
        val processHandler = OSProcessHandler(commandLine)
        ProcessTerminatedListener.attach(processHandler)

        val console: ConsoleView = ConsoleViewImpl(project, true)
        console.print("> glade exec --debug-log - <anonymous apex>\n", ConsoleViewContentType.SYSTEM_OUTPUT)
        console.attachToProcess(processHandler)

        val descriptor = RunContentDescriptor(
            console,
            processHandler,
            console.component,
            "Glade Execute Anonymous"
        )
        RunContentManager.getInstance(project).showRunContent(
            DefaultRunExecutor.getRunExecutorInstance(),
            descriptor
        )
        processHandler.startNotify()
    }
}
