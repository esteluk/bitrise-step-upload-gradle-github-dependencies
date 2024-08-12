package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
)

type Config struct {
	GradlewPath             string `env:"gradlew_path,file"`
	GithubAccessToken       string `env:"github_access_token,required"`
	IncludedProjects        string `env:"included_projects"`
	ExcludedProjects        string `env:"excluded_projects"`
	IncludedConfigs         string `env:"included_configurations"`
	ExcludedConfigs         string `env:"excluded_configurations"`
	RuntimeIncludedProjects string `env:"runtime_included_projects"`
	RuntimeExcludedProjects string `env:"runtime_excluded_projects"`
	RuntimeIncludedConfigs  string `env:"runtime_included_configurations"`
	RuntimeExcludedConfigs  string `env:"runtime_excluded_configurations"`
}

func failf(message string, args ...interface{}) {
	log.Errorf(message, args...)
	os.Exit(1)
}

func main() {
	// Parse out the step config
	var configs Config
	if err := stepconf.Parse(&configs); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(configs)

	log.Infof("Building gradle dependency graph")

	// Check that the gradlew file exists
	gradlewPath, err := filepath.Abs(configs.GradlewPath)
	if err != nil {
		failf("Can't get absolute path for gradlew file (%s): %s", configs.GradlewPath, err)
	}

	initScript, err := filepath.Abs(fmt.Sprintf("%s/graph-init-script.gradle"))
	if err != nil {
		failf("Can't find init script: %s", err)
	}

	cmdSlice := []string{gradlewPath}
	cmdSlice = append(
		cmdSlice, "-I", initScript,
		"--dependency-verification=off",
		"--no-configuration-cache",
		"--no-configure-on-demand",
		":ForceDependencyResolutionPlugin_resolveAllDependencies",
	)

	//  Included dependencies
	if configs.IncludedProjects != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_INCLUDE_PROJECTS=%s", configs.IncludedProjects))
	}
	if configs.ExcludedProjects != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_EXCLUDE_PROJECTS=%s", configs.ExcludedProjects))
	}
	if configs.IncludedConfigs != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_INCLUDE_CONFIGURATIONS=%s", configs.IncludedConfigs))
	}
	if configs.ExcludedConfigs != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_EXCLUDE_CONFIGURATIONS=%s", configs.ExcludedConfigs))
	}

	// Dependency scope
	if configs.RuntimeIncludedConfigs != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_RUNTIME_INCLUDE_PROJECTS=%s", configs.RuntimeIncludedProjects))
	}
	if configs.RuntimeExcludedProjects != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_RUNTIME_EXCLUDE_PROJECTS=%s", configs.RuntimeExcludedProjects))
	}
	if configs.RuntimeIncludedConfigs != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_RUNTIME_INCLUDE_CONFIGURATIONS=%s", configs.RuntimeIncludedConfigs))
	}
	if configs.RuntimeExcludedConfigs != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("-DDEPENDENCY_GRAPH_RUNTIME_EXCLUDE_CONFIGURATIONS=%s", configs.RuntimeExcludedConfigs))
	}

	_, err = exec.Command(cmdSlice[0], cmdSlice[1:]...).CombinedOutput()
	if err != nil {
		failf("Failed to execute Gradle command")
	}

	//
	// --- Step Outputs: Export Environment Variables for other Steps:
	// You can export Environment Variables for other Steps with
	//  envman, which is automatically installed by `bitrise setup`.
	// A very simple example:
	// cmdLog, err := exec.Command("bitrise", "envman", "add", "--key", "EXAMPLE_STEP_OUTPUT", "--value", "the value you want to share").CombinedOutput()
	// if err != nil {
	// 	fmt.Printf("Failed to expose output with envman, error: %#v | output: %s", err, cmdLog)
	// 	os.Exit(1)
	// }
	// You can find more usage examples on envman's GitHub page
	//  at: https://github.com/bitrise-io/envman

	//
	// --- Exit codes:
	// The exit code of your Step is very important. If you return
	//  with a 0 exit code `bitrise` will register your Step as "successful".
	// Any non zero exit code will be registered as "failed" by `bitrise`.
	os.Exit(0)
}
