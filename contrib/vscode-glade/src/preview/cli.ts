export function toolchainStatusArgs(): string[] {
  return ["toolchain", "status", "--json"];
}

export function toolchainStatusAllowedCodes(): Array<number | null> {
  return [0, 1];
}

export function toolchainInstallArgs(): string[] {
  return ["toolchain", "install"];
}

export function devLWCArgs(projectRoot: string, addr: string, readyFile: string): string[] {
  return ["dev", "lwc", "--project", projectRoot, "--addr", addr, "--ready-file", readyFile];
}

export function devVFArgs(projectRoot: string, addr: string, readyFile: string): string[] {
  return ["dev", "vf", "--project", projectRoot, "--addr", addr, "--ready-file", readyFile];
}
