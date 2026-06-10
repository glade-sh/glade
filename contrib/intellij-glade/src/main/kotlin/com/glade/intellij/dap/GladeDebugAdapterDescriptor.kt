package com.glade.intellij.dap

import com.glade.intellij.model.GladeSource
import com.glade.intellij.util.GladePaths
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.configurations.RunConfigurationOptions
import com.intellij.execution.process.ProcessHandler
import com.intellij.execution.runners.ExecutionEnvironment
import com.intellij.openapi.fileTypes.FileType
import com.redhat.devtools.lsp4ij.dap.DebugMode
import com.redhat.devtools.lsp4ij.dap.client.LaunchUtils
import com.redhat.devtools.lsp4ij.dap.configurations.DAPRunConfigurationOptions
import com.redhat.devtools.lsp4ij.dap.definitions.DebugAdapterServerDefinition
import com.redhat.devtools.lsp4ij.dap.descriptors.DebugAdapterDescriptor
import com.redhat.devtools.lsp4ij.dap.descriptors.ServerReadyConfig

class GladeDebugAdapterDescriptor(
    options: RunConfigurationOptions,
    environment: ExecutionEnvironment,
    serverDefinition: DebugAdapterServerDefinition?
) : DebugAdapterDescriptor(options, environment, serverDefinition) {
    override fun startServer(): ProcessHandler {
        val projectRoot = projectRoot()
        val commandLine = GeneralCommandLine("glade", "dap", "--project", projectRoot)
        commandLine.withWorkDirectory(projectRoot)
        return startServer(commandLine)
    }

    override fun getDapParameters(): Map<String, Any> {
        val dapOptions = options as? DAPRunConfigurationOptions ?: return mapOf(
            "type" to "glade",
            "name" to "Glade Apex",
            "request" to "launch",
            "project" to projectRoot(),
            "program" to "System.debug('hello from Glade');"
        )
        val raw = dapOptions.launchConfiguration
        if (raw.isNotBlank()) {
            val context = LaunchUtils.LaunchContext(dapOptions.file, dapOptions.workingDirectory)
            return LaunchUtils.getDapParameters(raw, context)
        }
        val projectRoot = projectRoot()
        val file = dapOptions.file.orEmpty()
        if (file.isNotBlank()) {
            return LaunchUtils.getDapParameters(
                GladeSource.launchJsonForProgram(projectRoot, file),
                LaunchUtils.LaunchContext(file, projectRoot)
            )
        }

        return mapOf(
            "type" to "glade",
            "name" to "Glade Apex",
            "request" to "launch",
            "project" to projectRoot(),
            "program" to "System.debug('hello from Glade');"
        )
    }

    override fun getDebugMode(): DebugMode = DebugMode.LAUNCH

    override fun getServerReadyConfig(debugMode: DebugMode): ServerReadyConfig {
        return ServerReadyConfig(0)
    }

    override fun getFileType(): FileType? = null

    private fun projectRoot(): String {
        val dapOptions = options as? DAPRunConfigurationOptions
        if (dapOptions != null) {
            val configured = dapOptions.workingDirectory
            if (!configured.isNullOrBlank()) {
                return configured
            }
        }
        return GladePaths.projectRoot(environment.project)
    }
}
