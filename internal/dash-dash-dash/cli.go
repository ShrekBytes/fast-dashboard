package dashdashdash

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type cliIntent uint8

const (
	cliIntentVersionPrint cliIntent = iota
	cliIntentServe
	cliIntentConfigValidate
	cliIntentConfigPrint
	cliIntentDiagnose
)

type cliOptions struct {
	intent     cliIntent
	configPath string
	args       []string
}

func parseCliOptions() (*cliOptions, error) {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		return &cliOptions{
			intent: cliIntentVersionPrint,
		}, nil
	}

	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Println("Usage: dash-dash-dash [options] command")

		fmt.Println("\nOptions:")
		flags.PrintDefaults()

		fmt.Println("\nCommands:")
		fmt.Println("  config:validate       Validate the config file")
		fmt.Println("  config:print          Print the parsed config file with embedded includes")
		fmt.Println("  diagnose              Run diagnostic checks")
	}

	configPath := flags.String("config", "config.yml", "Set config path")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	var intent cliIntent
	args = flags.Args()
	unknownCommandErr := fmt.Errorf("unknown command: %s", strings.Join(args, " "))

	if len(args) == 0 {
		intent = cliIntentServe
	} else if len(args) == 1 {
		switch args[0] {
		case "config:validate":
			intent = cliIntentConfigValidate
		case "config:print":
			intent = cliIntentConfigPrint
		case "diagnose":
			intent = cliIntentDiagnose
		default:
			return nil, unknownCommandErr
		}
	} else {
		return nil, unknownCommandErr
	}

	return &cliOptions{
		intent:     intent,
		configPath: *configPath,
		args:       args,
	}, nil
}

func cliDiagnose(configPath string) int {
	ok := true
	var config *config

	fmt.Println("Diagnostics:")
	fmt.Println()

	contents, _, err := parseYAMLIncludes(configPath)
	if err != nil {
		fmt.Printf(" ✗ Config file: %v\n", err)
		ok = false
	} else {
		fmt.Println(" ✓ Config file: found and includes resolved")
		config, err = newConfigFromYAML(contents)
		if err != nil {
			fmt.Printf(" ✗ Config parse/validate: %v\n", err)
			ok = false
		} else {
			fmt.Println(" ✓ Config parse/validate: OK")
		}
	}

	if config != nil {
		if config.Server.AssetsPath != "" {
			if _, err := os.Stat(config.Server.AssetsPath); err != nil {
				if os.IsNotExist(err) {
					fmt.Printf(" ✗ Assets path: directory does not exist: %s\n", config.Server.AssetsPath)
				} else {
					fmt.Printf(" ✗ Assets path: %v\n", err)
				}
				ok = false
			} else {
				fmt.Printf(" ✓ Assets path: %s\n", config.Server.AssetsPath)
			}
		}
	}

	fmt.Println()
	if ok {
		fmt.Println("All checks passed.")
		return 0
	}
	fmt.Println("Some checks failed.")
	return 1
}
