package dashdashdash

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func Main() int {
	options, err := parseCliOptions()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentVersionPrint:
		fmt.Printf("dash-dash-dash version %s\n", buildVersion)
	case cliIntentServe:
		if serveUpdateNoticeIfConfigLocationNotMigrated(options.configPath) {
			return 1
		}

		if err := serveApp(options.configPath); err != nil {
			fmt.Println(err)
			return 1
		}
	case cliIntentConfigValidate:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
			return 1
		}

		if _, err := newConfigFromYAML(contents); err != nil {
			fmt.Printf("Config file is invalid: %v\n", err)
			return 1
		}
		fmt.Println("Config is valid.")
	case cliIntentConfigPrint:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
			return 1
		}
		contents, err = parseConfigVariables(contents)
		if err != nil {
			fmt.Printf("Variable substitution failed: %v\n", err)
			return 1
		}
		fmt.Println(string(contents))
	case cliIntentDiagnose:
		return cliDiagnose(options.configPath)
	}

	return 0
}

func serveApp(configPath string) error {
	exitChannel := make(chan struct{})
	exitOnce := sync.Once{} // Prevent double close panic
	hadValidConfigOnStartup := false
	var stopServer func() error

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Fix goroutine leak
	
	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down", "signal", sig)
		if stopServer != nil {
			if err := stopServer(); err != nil {
				slog.Error("Error stopping server", "error", err)
			}
		}
		exitOnce.Do(func() { close(exitChannel) })
	}()

	onChange := func(newContents []byte) {
		if stopServer != nil {
			slog.Info("Config file changed, reloading...")
		}

		config, err := newConfigFromYAML(newContents)
		if err != nil {
			slog.Error("Config has errors", "error", err)

			if !hadValidConfigOnStartup {
				exitOnce.Do(func() { close(exitChannel) })
			}

			return
		}

		app, err := newApplication(config)
		if err != nil {
			slog.Error("Failed to create application", "error", err)

			if !hadValidConfigOnStartup {
				exitOnce.Do(func() { close(exitChannel) })
			}

			return
		}

		if !hadValidConfigOnStartup {
			hadValidConfigOnStartup = true
		}

		if stopServer != nil {
			if err := stopServer(); err != nil {
				slog.Error("Error while trying to stop server", "error", err)
			}
		}

		go func() {
			var startServer func() error
			startServer, stopServer = app.server()

			if err := startServer(); err != nil {
				slog.Error("Failed to start server", "error", err)
			}
		}()
	}

	onErr := func(err error) {
		slog.Error("Error watching config files", "error", err)
	}

	configContents, configIncludes, err := parseYAMLIncludes(configPath)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	stopWatching, err := configFilesWatcher(configPath, configContents, configIncludes, onChange, onErr)
	if err == nil {
		defer stopWatching()
	} else {
		slog.Warn("Error starting file watcher, config file changes will require a manual restart", "error", err)

		config, err := newConfigFromYAML(configContents)
		if err != nil {
			return fmt.Errorf("validating config file: %w", err)
		}

		app, err := newApplication(config)
		if err != nil {
			return fmt.Errorf("creating application: %w", err)
		}

		startServer, _ := app.server()
		if err := startServer(); err != nil {
			return fmt.Errorf("starting server: %w", err)
		}
	}

	<-exitChannel
	return nil
}

func serveUpdateNoticeIfConfigLocationNotMigrated(configPath string) bool {
	if !isRunningInsideDockerContainer() {
		return false
	}

	if _, err := os.Stat(configPath); err == nil {
		return false
	}

	// Warn if they have the old default filename (glance.yml) in CWD — they need to mount config at -config path
	if stat, err := os.Stat("glance.yml"); err != nil || stat.IsDir() {
		return false
	}

	fmt.Println("!!! WARNING !!!")
	fmt.Println("Default config path is config.yml.")
	fmt.Println("Please mount your config to the path specified by -config (e.g. /app/config/config.yml).")
	return true
}
