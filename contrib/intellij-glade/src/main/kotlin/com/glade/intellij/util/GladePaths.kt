package com.glade.intellij.util

import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile

object GladePaths {
    fun projectRoot(project: Project): String {
        return project.basePath ?: "."
    }

    fun isApexFile(file: VirtualFile?): Boolean {
        val name = file?.name ?: return false
        return name.endsWith(".cls", ignoreCase = true) || name.endsWith(".trigger", ignoreCase = true)
    }
}
