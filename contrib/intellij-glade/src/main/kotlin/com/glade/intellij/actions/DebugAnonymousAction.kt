package com.glade.intellij.actions

import com.glade.intellij.model.GladeSource
import com.glade.intellij.util.GladePaths
import com.intellij.execution.RunManager
import com.intellij.execution.RunnerAndConfigurationSettings
import com.intellij.execution.configurations.ConfigurationType
import com.intellij.execution.executors.DefaultDebugExecutor
import com.intellij.execution.ProgramRunnerUtil
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.actionSystem.PlatformDataKeys
import com.intellij.openapi.ui.Messages
import com.redhat.devtools.lsp4ij.dap.DebugMode
import com.redhat.devtools.lsp4ij.dap.configurations.DAPRunConfiguration

class DebugAnonymousAction : AnAction() {
    override fun update(event: AnActionEvent) {
        val file = event.getData(CommonDataKeys.VIRTUAL_FILE)
        event.presentation.isEnabledAndVisible = GladePaths.isApexFile(file)
    }

    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        val source = sourceFromEvent(event) ?: return
        val projectRoot = GladePaths.projectRoot(project)
        val settings = event.createTemporaryDapConfiguration(source, projectRoot)
        ProgramRunnerUtil.executeConfiguration(settings, DefaultDebugExecutor.getDebugExecutorInstance())
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
            "Enter anonymous Apex to debug with glade.",
            "Glade: Debug Anonymous Apex",
            "",
            null,
            null
        )
        return entered?.trim()?.ifEmpty { null }
    }

    private fun AnActionEvent.createTemporaryDapConfiguration(
        source: String,
        projectRoot: String
    ): RunnerAndConfigurationSettings {
        val project = project!!
        val type = ConfigurationType.CONFIGURATION_TYPE_EP.extensionList
            .first { it.id == "DAPConfiguration" }
        val factory = type.configurationFactories.first()
        val runManager = RunManager.getInstance(project)
        val settings = runManager.createConfiguration("Glade: Debug Anonymous Apex", factory)
        val configuration = settings.configuration as DAPRunConfiguration
        configuration.workingDirectory = projectRoot
        configuration.file = ""
        configuration.debugMode = DebugMode.LAUNCH
        configuration.serverId = "glade"
        configuration.serverName = "Glade Apex"
        configuration.launchConfiguration = GladeSource.launchJsonForSource(projectRoot, source)
        settings.isTemporary = true
        return settings
    }
}
