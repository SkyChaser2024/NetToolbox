package desktop

import "campusnet-toolbox/internal/settings"

type startupMigrationStore interface {
	NeedsStartupMigration() (bool, error)
	LoadConfiguration() (settings.Profile, settings.DiagnosticSettings, settings.SystemPreferences, error)
	SaveConfiguration(settings.Profile, settings.DiagnosticSettings, settings.SystemPreferences) (settings.Profile, settings.DiagnosticSettings, settings.SystemPreferences, error)
}

// Upgrade the existing scheduled task's action before marking the document as
// migrated. A failed operation leaves the legacy version for an idempotent retry.
func migrateLoginStartup(store startupMigrationStore, configure func(bool) error) error {
	needed, err := store.NeedsStartupMigration()
	if err != nil || !needed {
		return err
	}
	profile, diagnostics, system, err := store.LoadConfiguration()
	if err != nil {
		return err
	}
	if err := configure(system.StartAtLogin); err != nil {
		return err
	}
	_, _, _, err = store.SaveConfiguration(profile, diagnostics, system)
	return err
}
