package pluginhost

func FindByCommandRoot(state InstalledState, root string) (InstalledPlugin, bool) {
	for _, plugin := range state.Plugins {
		for _, command := range plugin.Commands {
			if command == root {
				return plugin, true
			}
		}
	}
	return InstalledPlugin{}, false
}
