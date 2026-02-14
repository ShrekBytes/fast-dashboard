package fastdashboard

import (
	"fmt"
	"log"
	"os"
)

func Main() int {
	options, err := parseCliOptions()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentVersionPrint:
		fmt.Println(buildVersion)
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
	hadValidConfigOnStartup := false
	var stopServer func() error

	onChange := func(newContents []byte) {
		if stopServer != nil {
			log.Println("Config file changed, reloading...")
		}

		config, err := newConfigFromYAML(newContents)
		if err != nil {
			log.Printf("Config has errors: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		}

		app, err := newApplication(config)
		if err != nil {
			log.Printf("Failed to create application: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		}

		if !hadValidConfigOnStartup {
			hadValidConfigOnStartup = true
		}

		if stopServer != nil {
			if err := stopServer(); err != nil {
				log.Printf("Error while trying to stop server: %v", err)
			}
		}

		go func() {
			var startServer func() error
			startServer, stopServer = app.server()

			if err := startServer(); err != nil {
				log.Printf("Failed to start server: %v", err)
			}
		}()
	}

	onErr := func(err error) {
		log.Printf("Error watching config files: %v", err)
	}

	configContents, configIncludes, err := parseYAMLIncludes(configPath)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	stopWatching, err := configFilesWatcher(configPath, configContents, configIncludes, onChange, onErr)
	if err == nil {
		defer stopWatching()
	} else {
		log.Printf("Error starting file watcher, config file changes will require a manual restart. (%v)", err)

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
