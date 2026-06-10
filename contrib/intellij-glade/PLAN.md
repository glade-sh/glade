# Glade IntelliJ IDE Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a JetBrains IDE plugin that lets users run and debug local Apex through the existing `glade` CLI.

**Architecture:** Keep all Apex execution and DAP protocol behavior in the Go product. The IntelliJ plugin stays thin: it registers Glade as an LSP4IJ DAP server, starts `glade dap --project <root>` over stdio, sends launch JSON with `program` or `source`, and adds editor actions that mirror the VS Code extension. Do not add a new DAP implementation, compatibility scanner, ledger, fixture runner, or maintenance command to this repo.

**Tech Stack:** Kotlin, IntelliJ Platform Gradle Plugin 2.x, IntelliJ IDEA Community platform, LSP4IJ DAP APIs, JUnit 5, GitHub Actions, existing `glade dap` and `glade exec`.

---

## Source Facts To Preserve

- `glade dap --project <root>` is the product DAP server. Its CLI entry is `internal/gladecli/dap_command.go`.
- DAP launch accepts `program`, `project`, and `source`. If `source` is present, Glade compiles that anonymous Apex. If `program` is a readable file, Glade reads it. If `program` is a bare entry point, Glade appends `();`.
- The VS Code extension starts DAP with `["dap", "--project", project]` and runs anonymous Apex with `["exec", "--debug-log", "-", source]`.
- The current IntelliJ Platform Gradle Plugin 2.x docs list minimums of IntelliJ Platform 2023.3, Gradle 9.0.0, and Java 17. Do not target IntelliJ 2023.2 with this build.
- JetBrains core IntelliJ docs do not expose a stable generic IntelliJ IDEA DAP API for third-party plugins. Use LSP4IJ, which exposes `com.redhat.devtools.lsp4ij.debugAdapterServer`, `DebugAdapterDescriptorFactory`, `DebugAdapterDescriptor`, `DAPRunConfiguration`, and `DAPConfiguration`.
- LSP4IJ 0.19.4 is the pinned dependency for this plan. Upgrade only in a separate commit after a green baseline.

## Boundaries

- Keep this work under `contrib/intellij-glade/` and `.github/workflows/intellij-glade.yml`.
- Do not change `internal/dap`, `internal/gladecli`, or VM code unless a focused DAP smoke test proves the product server is wrong.
- Do not add a product dependency on `glade-tools`.
- Do not add a custom `XDebugProcess`.
- Do not register `.cls` or `.trigger` as an Apex file type in the first cut. That can conflict with existing JetBrains Apex plugins. Actions must enable by file extension instead.
- Do not commit built plugin zips, Gradle caches, `.idea/`, or sandbox directories.

## File Map

- Create `contrib/intellij-glade/settings.gradle.kts`: Gradle plugin repositories.
- Create `contrib/intellij-glade/build.gradle.kts`: Kotlin plugin build, LSP4IJ dependency, test task, plugin verification.
- Create `contrib/intellij-glade/gradle.properties`: pinned versions and plugin metadata.
- Create `contrib/intellij-glade/src/main/resources/META-INF/plugin.xml`: plugin id, LSP4IJ dependency, DAP server extension, editor actions.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/dap/GladeDebugAdapterDescriptor.kt`: starts `glade dap` and returns launch parameters.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/dap/GladeDebugAdapterDescriptorFactory.kt`: registers launch snippets and file eligibility.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/ExecuteAnonymousAction.kt`: runs `glade exec --debug-log - <source>` in a Run console.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/DebugAnonymousAction.kt`: creates a temporary LSP4IJ DAP run configuration and starts the debug executor.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/model/GladeSource.kt`: source extraction and launch JSON escaping.
- Create `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/util/GladePaths.kt`: project root and extension checks.
- Create `contrib/intellij-glade/src/test/kotlin/com/glade/intellij/model/GladeSourceTest.kt`: pure unit tests for source extraction and launch JSON.
- Create `contrib/intellij-glade/src/test/kotlin/com/glade/intellij/dap/GladeDapProtocolTest.kt`: wire-level smoke test against the local `glade dap`.
- Create `.github/workflows/intellij-glade.yml`: isolated plugin CI.
- Modify `.gitignore`: ignore IntelliJ plugin Gradle and sandbox outputs.

## Task 1: Scaffold The IntelliJ Plugin Build

**Files:**
- Create: `contrib/intellij-glade/settings.gradle.kts`
- Create: `contrib/intellij-glade/build.gradle.kts`
- Create: `contrib/intellij-glade/gradle.properties`
- Create: `contrib/intellij-glade/src/main/resources/META-INF/plugin.xml`
- Modify: `.gitignore`

- [ ] **Step 1: Create `settings.gradle.kts`**

```kotlin
import org.jetbrains.intellij.platform.gradle.extensions.intellijPlatform

pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}

plugins {
    id("org.jetbrains.intellij.platform.settings") version "2.16.0"
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
        intellijPlatform {
            defaultRepositories()
            marketplace()
        }
    }
}

rootProject.name = "intellij-glade"
```

- [ ] **Step 2: Create `gradle.properties`**

```properties
pluginGroup=com.glade.intellij
pluginName=Glade Apex Debugger
pluginVersion=0.1.0
pluginSinceBuild=253
pluginUntilBuild=261.*
platformType=IC
platformVersion=2025.3
kotlinVersion=2.2.21
intellijPlatformPluginVersion=2.16.0
lsp4ijVersion=0.19.4
org.gradle.jvmargs=-Xmx2g -Dfile.encoding=UTF-8
```

- [ ] **Step 3: Create `build.gradle.kts`**

```kotlin
import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "2.2.21"
    id("org.jetbrains.intellij.platform") version "2.16.0"
}

group = providers.gradleProperty("pluginGroup").get()
version = providers.gradleProperty("pluginVersion").get()

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(17))
    }
}

kotlin {
    jvmToolchain(17)
}

dependencies {
    intellijPlatform {
        create(
            IntelliJPlatformType.IntellijIdeaCommunity,
            providers.gradleProperty("platformVersion").get()
        )
        bundledPlugin("com.intellij.java")
        plugin("com.redhat.devtools.lsp4ij", providers.gradleProperty("lsp4ijVersion").get())
        testFramework(TestFrameworkType.Platform)
    }

    testImplementation(kotlin("test"))
}

intellijPlatform {
    pluginConfiguration {
        name = providers.gradleProperty("pluginName")
        version = providers.gradleProperty("pluginVersion")
        ideaVersion {
            sinceBuild = providers.gradleProperty("pluginSinceBuild")
            untilBuild = providers.gradleProperty("pluginUntilBuild")
        }
    }

    pluginVerification {
        ides {
            recommended()
        }
    }
}

tasks {
    test {
        useJUnitPlatform()
    }
}
```

- [ ] **Step 4: Create the initial `plugin.xml`**

```xml
<idea-plugin>
    <id>com.glade.intellij</id>
    <name>Glade Apex Debugger</name>
    <vendor email="support@glade.sh" url="https://github.com/glade-sh/glade">Glade</vendor>

    <description><![CDATA[
        Run and debug local Apex with Glade.
    ]]></description>

    <depends>com.intellij.modules.platform</depends>
    <depends>com.intellij.modules.lang</depends>
    <depends>com.redhat.devtools.lsp4ij</depends>

    <extensions defaultExtensionNs="com.redhat.devtools.lsp4ij">
        <debugAdapterServer
            id="glade"
            name="Glade Apex"
            factoryClass="com.glade.intellij.dap.GladeDebugAdapterDescriptorFactory" />
    </extensions>

    <actions>
        <action
            id="Glade.ExecuteAnonymous"
            class="com.glade.intellij.actions.ExecuteAnonymousAction"
            text="Glade: Execute Anonymous Apex"
            description="Execute anonymous Apex with glade">
            <add-to-group group-id="EditorPopupMenu" anchor="last"/>
        </action>
        <action
            id="Glade.DebugAnonymous"
            class="com.glade.intellij.actions.DebugAnonymousAction"
            text="Glade: Debug Anonymous Apex"
            description="Debug anonymous Apex with glade">
            <add-to-group group-id="EditorPopupMenu" anchor="last"/>
        </action>
    </actions>
</idea-plugin>
```

- [ ] **Step 5: Extend `.gitignore`**

Add these lines if they are not present:

```gitignore
contrib/intellij-glade/.gradle/
contrib/intellij-glade/.intellijPlatform/
contrib/intellij-glade/build/
contrib/intellij-glade/out/
contrib/intellij-glade/sandbox/
contrib/intellij-glade/*.iml
```

- [ ] **Step 6: Create a Gradle wrapper**

Run:

```bash
cd contrib/intellij-glade
gradle wrapper --gradle-version 9.0.0
```

Expected:

```text
BUILD SUCCESSFUL
```

If `gradle` is not installed, install it outside the repo, then rerun the command. Do not vendor Gradle manually.

- [ ] **Step 7: Run the first build**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon clean buildPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

If this fails on the LSP4IJ dependency, stop and record the exact Gradle error in this plan under a new `Build Drift Notes` section. Do not switch to JetBrains core DAP APIs.

- [ ] **Step 8: Commit**

```bash
git add .gitignore contrib/intellij-glade
git commit -m "chore: scaffold IntelliJ Glade plugin"
```

## Task 2: Add Source And Path Utilities

**Files:**
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/model/GladeSource.kt`
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/util/GladePaths.kt`
- Create: `contrib/intellij-glade/src/test/kotlin/com/glade/intellij/model/GladeSourceTest.kt`

- [ ] **Step 1: Write `GladeSourceTest.kt`**

```kotlin
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
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon test --tests com.glade.intellij.model.GladeSourceTest
```

Expected:

```text
Unresolved reference 'GladeSource'
```

- [ ] **Step 3: Create `GladeSource.kt`**

```kotlin
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
```

- [ ] **Step 4: Create `GladePaths.kt`**

```kotlin
package com.glade.intellij.util

import com.intellij.openapi.project.Project
import com.intellij.openapi.project.ProjectUtil
import com.intellij.openapi.vfs.VirtualFile

object GladePaths {
    fun projectRoot(project: Project): String {
        return ProjectUtil.guessProjectDir(project)?.path ?: project.basePath ?: "."
    }

    fun isApexFile(file: VirtualFile?): Boolean {
        val name = file?.name ?: return false
        return name.endsWith(".cls", ignoreCase = true) || name.endsWith(".trigger", ignoreCase = true)
    }
}
```

- [ ] **Step 5: Run the passing test**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon test --tests com.glade.intellij.model.GladeSourceTest
```

Expected:

```text
BUILD SUCCESSFUL
```

- [ ] **Step 6: Commit**

```bash
git add contrib/intellij-glade/src/main/kotlin contrib/intellij-glade/src/test/kotlin
git commit -m "test: add IntelliJ Glade source utilities"
```

## Task 3: Register Glade As An LSP4IJ DAP Server

**Files:**
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/dap/GladeDebugAdapterDescriptor.kt`
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/dap/GladeDebugAdapterDescriptorFactory.kt`
- Test: `./gradlew --no-daemon buildPlugin`

- [ ] **Step 1: Create `GladeDebugAdapterDescriptor.kt`**

```kotlin
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
        if (options is DAPRunConfigurationOptions) {
            val raw = options.launchConfiguration
            if (raw.isNotBlank()) {
                val context = LaunchUtils.LaunchContext(options.file, options.workingDirectory)
                return LaunchUtils.getDapParameters(raw, context)
            }
            val projectRoot = projectRoot()
            val file = options.file.orEmpty()
            if (file.isNotBlank()) {
                return LaunchUtils.getDapParameters(
                    GladeSource.launchJsonForProgram(projectRoot, file),
                    LaunchUtils.LaunchContext(file, projectRoot)
                )
            }
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
        if (options is DAPRunConfigurationOptions) {
            val configured = options.workingDirectory
            if (!configured.isNullOrBlank()) {
                return configured
            }
        }
        return GladePaths.projectRoot(environment.project)
    }
}
```

- [ ] **Step 2: Create `GladeDebugAdapterDescriptorFactory.kt`**

```kotlin
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
```

- [ ] **Step 3: Compile the plugin**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon clean buildPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

If Kotlin reports a changed LSP4IJ symbol, open the matching class from the local Gradle dependency cache or from `https://github.com/redhat-developer/lsp4ij/tree/0.19.4`, adjust only the import or method signature, and rerun this same command. Do not replace LSP4IJ with a hand-written DAP client.

- [ ] **Step 4: Manual sandbox check**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon runIde
```

Expected in the sandbox IDE:

- Settings, Build, Execution, Deployment, Debugger, DAP Debuggers shows `Glade Apex`.
- A DAP run configuration can choose server `Glade Apex`.
- Starting that configuration launches `glade dap --project <projectRoot>`.

- [ ] **Step 5: Commit**

```bash
git add contrib/intellij-glade
git commit -m "feat: register Glade DAP server for IntelliJ"
```

## Task 4: Add Execute Anonymous Action

**Files:**
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/ExecuteAnonymousAction.kt`
- Test: `./gradlew --no-daemon buildPlugin`

- [ ] **Step 1: Create `ExecuteAnonymousAction.kt`**

```kotlin
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
import com.intellij.execution.ui.ConsoleViewImpl
import com.intellij.execution.ui.RunContentDescriptor
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

        val descriptor = RunContentDescriptor(console, processHandler, console.component, "Glade Execute Anonymous")
        ExecutionManager.getInstance(project).contentManager.showRunContent(
            DefaultRunExecutor.getRunExecutorInstance(),
            descriptor
        )
        processHandler.startNotify()
    }
}
```

- [ ] **Step 2: Compile**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon buildPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

- [ ] **Step 3: Manual sandbox check**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon runIde
```

Expected:

- Right-clicking a `.cls` or `.trigger` editor shows `Glade: Execute Anonymous Apex`.
- Selecting `System.debug('hello');` and running the action opens a Run tab.
- The Run tab prints a Salesforce-style debug log from `glade exec --debug-log -`.

- [ ] **Step 4: Commit**

```bash
git add contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/ExecuteAnonymousAction.kt
git commit -m "feat: add IntelliJ execute anonymous action"
```

## Task 5: Add Debug Anonymous Action

**Files:**
- Create: `contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/DebugAnonymousAction.kt`
- Test: `./gradlew --no-daemon buildPlugin`

- [ ] **Step 1: Create `DebugAnonymousAction.kt`**

```kotlin
package com.glade.intellij.actions

import com.glade.intellij.model.GladeSource
import com.glade.intellij.util.GladePaths
import com.intellij.execution.RunManager
import com.intellij.execution.RunnerAndConfigurationSettings
import com.intellij.execution.configurations.ConfigurationType
import com.intellij.execution.executors.DefaultDebugExecutor
import com.intellij.execution.runners.ProgramRunnerUtil
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
```

- [ ] **Step 2: Compile**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon buildPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

If `ConfigurationType.CONFIGURATION_TYPE_EP.extensionList` has changed in the target platform, use IntelliJ's public configuration type extension list for the same id, `DAPConfiguration`. Keep the cast to `DAPRunConfiguration`.

- [ ] **Step 3: Manual sandbox check**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon runIde
```

Expected:

- Right-clicking a `.cls` or `.trigger` editor shows `Glade: Debug Anonymous Apex`.
- Selecting Apex source and running the action opens a Debug tool window.
- LSP4IJ starts `glade dap --project <projectRoot>`.
- Variables and stack frames come from Glade DAP responses.

- [ ] **Step 4: Commit**

```bash
git add contrib/intellij-glade/src/main/kotlin/com/glade/intellij/actions/DebugAnonymousAction.kt
git commit -m "feat: add IntelliJ debug anonymous action"
```

## Task 6: Add Wire-Level DAP Smoke Test

**Files:**
- Create: `contrib/intellij-glade/src/test/kotlin/com/glade/intellij/dap/GladeDapProtocolTest.kt`

- [ ] **Step 1: Write `GladeDapProtocolTest.kt`**

```kotlin
package com.glade.intellij.dap

import java.io.InputStream
import java.io.OutputStream
import java.nio.charset.StandardCharsets
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
            val disconnect = reader.readMessage()
            assertTrue(disconnect.contains(""""command":"disconnect""""))

            val terminated = reader.readMessage()
            assertTrue(terminated.contains(""""event":"terminated""""))
            assertEquals(0, process.waitFor())
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
    }
}
```

- [ ] **Step 2: Run the focused test**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon test --tests com.glade.intellij.dap.GladeDapProtocolTest
```

Expected:

```text
BUILD SUCCESSFUL
```

- [ ] **Step 3: Run the product-side DAP tests**

Run:

```bash
go test ./internal/dap ./internal/gladecli
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/dap
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 4: Commit**

```bash
git add contrib/intellij-glade/src/test/kotlin/com/glade/intellij/dap/GladeDapProtocolTest.kt
git commit -m "test: add IntelliJ DAP wire smoke"
```

## Task 7: Add CI

**Files:**
- Create: `.github/workflows/intellij-glade.yml`

- [ ] **Step 1: Create the workflow**

```yaml
name: intellij-glade

on:
  push:
    paths:
      - "contrib/intellij-glade/**"
      - ".github/workflows/intellij-glade.yml"
      - "internal/dap/**"
      - "internal/gladecli/**"
  pull_request:
    paths:
      - "contrib/intellij-glade/**"
      - ".github/workflows/intellij-glade.yml"
      - "internal/dap/**"
      - "internal/gladecli/**"

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: "17"

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build and test plugin
        working-directory: contrib/intellij-glade
        run: ./gradlew --no-daemon clean test buildPlugin

      - name: Verify product DAP packages
        run: go test ./internal/dap ./internal/gladecli
```

- [ ] **Step 2: Run CI commands locally**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon clean test buildPlugin
cd ../..
go test ./internal/dap ./internal/gladecli
```

Expected:

```text
BUILD SUCCESSFUL
ok  	github.com/glade-sh/glade/internal/dap
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/intellij-glade.yml
git commit -m "ci: add IntelliJ Glade plugin build"
```

## Task 8: Add Plugin Docs

**Files:**
- Create: `contrib/intellij-glade/README.md`
- Create: `contrib/intellij-glade/CHANGELOG.md`

- [ ] **Step 1: Create `README.md`**

```markdown
# Glade Apex Debugger for IntelliJ

This plugin lets JetBrains IDE users run and debug local Apex through the `glade` CLI.

## Requirements

- IntelliJ Platform build 253 or newer.
- LSP4IJ 0.19.4, installed as a plugin dependency.
- A `glade` binary on `PATH`.
- A local SFDX-style project when debugging project classes.

## Build

```bash
./gradlew --no-daemon clean test buildPlugin
```

## Run In A Sandbox IDE

```bash
./gradlew --no-daemon runIde
```

## Execute Anonymous Apex

Open a `.cls` or `.trigger` file, select Apex source, then choose `Glade: Execute Anonymous Apex`.

The plugin runs:

```bash
glade exec --debug-log - <selected source>
```

## Debug Anonymous Apex

Open a `.cls` or `.trigger` file, select Apex source, then choose `Glade: Debug Anonymous Apex`.

The plugin creates a temporary LSP4IJ DAP configuration and starts:

```bash
glade dap --project <project root>
```

The launch request sends the selected source in the DAP `source` field.

## Notes

The plugin does not register an Apex parser or claim `.cls` and `.trigger` file types. It enables actions by file extension to avoid conflicting with existing JetBrains Apex plugins.
```

- [ ] **Step 2: Create `CHANGELOG.md`**

```markdown
# Changelog

## 0.1.0

- Added LSP4IJ DAP server registration for `glade dap`.
- Added execute-anonymous editor action backed by `glade exec --debug-log -`.
- Added debug-anonymous editor action backed by an LSP4IJ DAP run configuration.
- Added focused DAP wire smoke coverage.
```

- [ ] **Step 3: Commit**

```bash
git add contrib/intellij-glade/README.md contrib/intellij-glade/CHANGELOG.md
git commit -m "docs: add IntelliJ Glade plugin guide"
```

## Task 9: Final Verification

**Files:**
- No new files.

- [ ] **Step 1: Run product DAP package tests**

Run:

```bash
go test ./internal/dap ./internal/gladecli
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/dap
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 2: Run plugin tests and build**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon clean test buildPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

- [ ] **Step 3: Run plugin verifier**

Run:

```bash
cd contrib/intellij-glade
./gradlew --no-daemon verifyPlugin
```

Expected:

```text
BUILD SUCCESSFUL
```

- [ ] **Step 4: Check git status**

Run:

```bash
git status --short
```

Expected: only intentional files from this plan are modified or added.

## Acceptance Criteria

- `contrib/intellij-glade` builds with Gradle 9 and Java 17.
- LSP4IJ sees a `Glade Apex` DAP server with id `glade`.
- `GladeDebugAdapterDescriptor` starts `glade dap --project <projectRoot>` over stdio.
- Launch parameters include `type=glade`, `request=launch`, `project`, and either `program` or `source`.
- Execute Anonymous runs `glade exec --debug-log - <source>` and shows output in a Run tab.
- Debug Anonymous opens the Debug tool window through LSP4IJ and starts a temporary DAP configuration.
- Focused tests pass: `GladeSourceTest`, `GladeDapProtocolTest`, `go test ./internal/dap ./internal/gladecli`.
- No new maintenance commands or `glade-tools` dependencies enter the product repo.

## Known Risks And Responses

| Risk | Response |
| --- | --- |
| LSP4IJ API drift | Keep Task 3 as the compile gate. Update imports or method signatures from the pinned `0.19.4` source only. |
| Existing Apex plugin conflicts | Do not register `.cls` or `.trigger` file types in this plan. Enable actions by extension. |
| `glade` missing from PATH | LSP4IJ run output will show the process start failure. Add a user-facing PATH setting only after this thin path works. |
| Gradle verifier matrix cost | Start with `recommended()` verifier IDEs. Widen the matrix after the first Marketplace-ready artifact. |
| Debug Anonymous API drift | Use the public LSP4IJ `DAPRunConfiguration` setters and the platform configuration type id `DAPConfiguration`. |

## References

- IntelliJ Platform Gradle Plugin 2.x docs: `https://plugins.jetbrains.com/docs/intellij/tools-intellij-platform-gradle-plugin.html`
- IntelliJ Run Configurations docs: `https://plugins.jetbrains.com/docs/intellij/run-configurations.html`
- LSP4IJ DAP developer guide: `https://github.com/redhat-developer/lsp4ij/blob/main/docs/dap/DeveloperGuide.md`
- LSP4IJ 0.19.4 release: `https://github.com/redhat-developer/lsp4ij/releases/tag/0.19.4`
- Glade VS Code adapter reference: `contrib/vscode-glade/src/adapter.ts`
- Glade VS Code command reference: `contrib/vscode-glade/src/commandModel.ts`
- Glade DAP CLI reference: `internal/gladecli/dap_command.go`
