package desktop

import "campusnet-toolbox/internal/settings"

// Silent startup applies to a new session. Explicit restores and subsequent
// manual launches must still provide a way to open the settings window.
func trySilentStartup(arguments []string, prefs settings.SystemPreferences, trayRunning, foregroundRunning bool, run func() error) (bool, error) {
	if hasArgument(arguments, "--restore-overview") {
		return false, nil
	}
	if hasArgument(arguments, "--login") && (trayRunning || foregroundRunning) {
		return true, nil
	}
	if !prefs.SilentStart {
		return false, nil
	}
	if trayRunning || foregroundRunning {
		return false, nil
	}
	err := run()
	return err == nil, err
}

func needsBackground(prefs settings.SystemPreferences) bool {
	return prefs.AutoAuthenticate || prefs.CloseToTray || prefs.SilentStart
}
