package com.glade.intellij.dap

import com.glade.intellij.model.GladeSource
import com.glade.intellij.util.GladePaths
import com.intellij.execution.configurations.RunConfiguration
import com.intellij.execution.configurations.RunConfigurationOptions
import com.intellij.execution.runners.ExecutionEnvironment
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.redhat.devtools.lsp4ij.dap.DebugMode
import com.redhat.devtools.lsp4ij.dap.LaunchConfiguration
import com.redhat.devtools.lsp4ij.dap.configurations.DAPRunConfiguration
import com.redhat.devtools.lsp4ij.dap.descriptors.DebugAdapterDescriptor
import com.redhat.devtools.lsp4ij.dap.descriptors.DebugAdapterDescriptorFactory

class GladeDebugAdapterDescriptorFactory : DebugAdapterDescriptorFactory() {
    override fun createDebugAdapterDescriptor(
        options: RunConfigurationOptions,
        environment: ExecutionEnvironment
    ): DebugAdapterDescriptor {
        return GladeDebugAdapterDescriptor(options, environment, serverDefinition)
    }

    override fun isDebuggableFile(file: VirtualFile, project: Project): Boolean {
        return GladePaths.isApexFile(file)
    }

    override fun prepareConfiguration(
        configuration: RunConfiguration,
        file: VirtualFile,
        project: Project
    ): Boolean {
        if (!GladePaths.isApexFile(file)) {
            return false
        }
        if (configuration is DAPRunConfiguration) {
            val projectRoot = GladePaths.projectRoot(project)
            configuration.name = "Glade: ${file.name}"
            configuration.file = file.path
            configuration.workingDirectory = projectRoot
            configuration.debugMode = DebugMode.LAUNCH
            configuration.serverId = serverDefinition.id
            configuration.serverName = serverDefinition.name
            configuration.launchConfiguration = GladeSource.launchJsonForProgram(projectRoot, file.path)
            return true
        }
        return false
    }

    override fun getLaunchConfigurations(): List<LaunchConfiguration> {
        return listOf(
            LaunchConfiguration(
                "glade_file",
                "Launch Apex file",
                """
                {
                  "type": "glade",
                  "name": "Glade Apex",
                  "request": "launch",
                  "project": "${'$'}{workspaceFolder}",
                  "program": "${'$'}{file}"
                }
                """.trimIndent(),
                DebugMode.LAUNCH
            )
        )
    }
}
