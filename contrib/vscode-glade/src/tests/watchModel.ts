import { GladeProjectContext } from "../projectModel";

export function watchArgs(project: GladeProjectContext): string[] {
  return ["test", "--project", project.projectRoot, "--daemon", "--watch"];
}
