package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
)

type Config struct {
	GradlewPath             string `env:"gradlew_path,file"`
	GithubToken             string `env:"github_token,required"`
	GithubOwner             string `env:"github_owner",required`
	GithubRepo              string `env:"github_repo",required`
	GithubJobCorrelator     string `env:"github_job_correlator",required`
	GithubJobId             string `env:"github_job_id",required`
	GithubGraphRef          string `env:"github_graph_ref",required`
	GithubGraphSha          string `env:"github_graph_sha",required`
	GithubGraphWorkspace    string `env:"github_graph_workspace",required`
	GithubSha               string `env:"github_graph_sha",required`
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

	initScript, err := filepath.Abs(fmt.Sprintf("%s/graph-init-script.gradle", os.Getenv("BITRISE_STEP_SOURCE_DIR")))
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

	cmdSlice = append(cmdSlice, fmt.Sprintf("-DGITHUB_DEPENDENCY_GRAPH_JOB_CORRELATOR=%s", configs.GithubJobCorrelator))
	cmdSlice = append(cmdSlice, fmt.Sprintf("-DGITHUB_DEPENDENCY_GRAPH_JOB_ID=%s", configs.GithubJobId))
	cmdSlice = append(cmdSlice, fmt.Sprintf("-DGITHUB_DEPENDENCY_GRAPH_REF=%s", configs.GithubGraphRef))
	cmdSlice = append(cmdSlice, fmt.Sprintf("-DGITHUB_DEPENDENCY_GRAPH_SHA=%s", configs.GithubSha))
	cmdSlice = append(cmdSlice, fmt.Sprintf("-DGITHUB_DEPENDENCY_GRAPH_WORKSPACE=%s", configs.GithubGraphWorkspace))

	cmd := command.New(cmdSlice[0], cmdSlice[1:]...)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)

	err = cmd.Run()
	if err != nil {
		failf("Failed to execute Gradle command: %s", err)
	}

	graphLocation, err := filepath.Abs(fmt.Sprintf("build/reports/dependency-graph-snapshots/%s.json", configs.GithubJobCorrelator))
	if err != nil {
		failf("Failed to generate dependency graph: %s", err)
	}

	log.Infof("Built dependency graph at %s", graphLocation)

	// Export output environment variable
	cmd = command.New("bitrise", "envman", "add", "--key", "GITHUB_DEPENDENCY_GRAPH", "--value", graphLocation)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	err = cmd.Run()
	if err != nil {
		failf("Failed to expose output with envman, error: %#v", err)
	}

	// Send to Github

	file, err := os.Open(graphLocation)
	if err != nil {
		failf("Failed to open dependency graph: %s", err)
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dependency-graph/snapshots", configs.GithubOwner, configs.GithubRepo)
	req, err := http.NewRequest("POST", url, file)
	if err != nil {
		failf("Failed to create Github HTTP Request: %s", err)
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", configs.GithubToken))

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		failf("Error when making HTTP request: %s", err)
	}
	defer response.Body.Close()

	var j interface{}
	err = json.NewDecoder(response.Body).Decode(&j)
	if err != nil {
		failf("Failed to decode JSON response: %s", err)
	}
	result := j.(map[string]interface{})["result"]
	if result == "ACCEPTED" || result == "SUCCESS" {
		log.Infof("Successfully uploaded dependency graph")
	} else {
		failf("Failed to submit graph to Github. Received response: %s", j)
	}

	os.Exit(0)
}
