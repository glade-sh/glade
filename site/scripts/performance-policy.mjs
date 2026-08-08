export function performancePolicy(runner, currentPlatform) {
  const enforceTimingBudgets = runner.platform === currentPlatform
  return {
    runs: enforceTimingBudgets ? runner.runs : 1,
    enforceTimingBudgets
  }
}
