package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v44/github"
	"github.com/magefile/mage/sh"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/klog/v2"
)

var (
	requiredBinaries = []string{"jq", "kubectl", "oc", "yq", "git"}
	artifactDir      = utils.GetEnv("ARTIFACT_DIR", ".")
	openshiftJobSpec = &OpenshiftJobSpec{}
	pr               = &PullRequestMetadata{}
	jobName          = utils.GetEnv("JOB_NAME", "")
	// can be periodic, presubmit or postsubmit
	jobType                    = utils.GetEnv("JOB_TYPE", "")
	reposToDeleteDefaultRegexp = "jvm-build-suite|e2e-dotnet|build-suite-test|e2e-multiple-components|e2e-nodejs|pet-clinic-e2e|test-app|multi-component-application|e2e-quayio|petclinic"
)

func (CI) parseJobSpec() error {
	jobSpecEnvVarData := os.Getenv("JOB_SPEC")

	if err := json.Unmarshal([]byte(jobSpecEnvVarData), openshiftJobSpec); err != nil {
		return fmt.Errorf("error when parsing openshift job spec data: %v", err)
	}
	return nil
}

func (ci CI) init() error {
	var err error

	if jobType == "periodic" || strings.Contains(jobName, "rehearse") {
		return nil
	}

	if err = ci.parseJobSpec(); err != nil {
		return err
	}

	pr.Author = openshiftJobSpec.Refs.Pulls[0].Author
	pr.Organization = openshiftJobSpec.Refs.Organization
	pr.RepoName = openshiftJobSpec.Refs.Repo
	pr.CommitSHA = openshiftJobSpec.Refs.Pulls[0].SHA
	pr.Number = openshiftJobSpec.Refs.Pulls[0].Number

	prUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", pr.Organization, pr.RepoName, pr.Number)
	pr.RemoteName, pr.BranchName, err = getRemoteAndBranchNameFromPRLink(prUrl)
	if err != nil {
		return err
	}

	return nil
}

func (ci CI) PrepareE2EBranch() error {
	if jobType == "periodic" || strings.Contains(jobName, "rehearse") {
		return nil
	}

	if err := ci.init(); err != nil {
		return err
	}

	if openshiftJobSpec.Refs.Repo == "e2e-tests" {
		if err := gitCheckoutRemoteBranch(pr.Author, pr.CommitSHA); err != nil {
			return err
		}
	} else {
		if ci.isPRPairingRequired() {
			if err := gitCheckoutRemoteBranch(pr.Author, pr.BranchName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (Local) PrepareCluster() error {
	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}
	if err := BootstrapCluster(); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}

	return nil
}

func (Local) TestE2E() error {
	return RunE2ETests()
}

// Deletes autogenerated repositories from redhat-appstudio-qe Github org.
// Env vars to configure this target: REPO_REGEX (optional), DRY_RUN (optional) - defaults to false
func (Local) CleanupGithubOrg() error {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		return fmt.Errorf("env var GITHUB_TOKEN is not set")
	}
	dryRun, err := strconv.ParseBool(utils.GetEnv("DRY_RUN", "true"))
	if err != nil {
		return fmt.Errorf("unable to parse DRY_RUN env var\n\t%s", err)
	}

	// Get all repos
	ghClient := github.NewGithubClient(githubToken, "redhat-appstudio-qe")
	repos, err := ghClient.GetAllRepositories()
	if err != nil {
		return err
	}
	var reposToDelete []*gh.Repository

	// Filter repos by regex & time check
	r, err := regexp.Compile(utils.GetEnv("REPO_REGEX", reposToDeleteDefaultRegexp))
	if err != nil {
		return fmt.Errorf("unable to compile regex: %s", err)
	}
	for _, repo := range repos {
		// Add only repos older than 24 hours
		dayDuration, _ := time.ParseDuration("24h")
		if time.Since(repo.GetCreatedAt().Time) > dayDuration {
			// Add only repos machting the regex
			if r.MatchString(*repo.Name) {
				reposToDelete = append(reposToDelete, repo)
			}
		}
	}

	if dryRun {
		klog.Info("Dry run enabled. Listing repositories that would be deleted:")
	}

	// Delete repos
	for _, repo := range reposToDelete {
		if dryRun {
			klog.Infof("\t%s", repo.GetName())
		} else {
			err := ghClient.DeleteRepository(repo)
			if err != nil {
				klog.Warningf("error deleting repository: %s\n", err)
			}
		}
	}
	if dryRun {
		klog.Info("If you really want to delete these repositories, run `DRY_RUN=false [REGEXP=<regexp>] mage local:cleanupGithubOrg`")
	}
	return nil
}

func (ci CI) TestE2E() error {
	var testFailure bool

	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}

	if err := ci.setRequiredEnvVars(); err != nil {
		return fmt.Errorf("error when setting up required env vars: %v", err)
	}

	if err := retry(BootstrapCluster, 2, 10*time.Second); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}

	if err := RunE2ETests(); err != nil {
		testFailure = true
	}

	if err := ci.sendWebhook(); err != nil {
		klog.Infof("error when sending webhook: %v", err)
	}

	if testFailure {
		return fmt.Errorf("error when running e2e tests - see the log above for more details")
	}

	return nil
}

func RunE2ETests() error {
	cwd, _ := os.Getwd()

	return sh.RunV("ginkgo", "-p", "--timeout=90m", fmt.Sprintf("--output-dir=%s", artifactDir), "--junit-report=e2e-report.xml", "--v", "--progress", "--label-filter=$E2E_TEST_SUITE_LABEL", "./cmd", "--", fmt.Sprintf("--config-suites=%s/tests/e2e-demos/config/default.yaml", cwd))
}

func PreflightChecks() error {
	if os.Getenv("GITHUB_TOKEN") == "" || os.Getenv("QUAY_TOKEN") == "" {
		return fmt.Errorf("required env vars containing secrets (QUAY_TOKEN, GITHUB_TOKEN) not defined or empty")
	}

	for _, binaryName := range requiredBinaries {
		if err := sh.Run("which", binaryName); err != nil {
			return fmt.Errorf("binary %s not found in PATH - please install it first", binaryName)
		}
	}

	if err := sh.RunV("go", "install", "-mod=mod", "github.com/onsi/ginkgo/v2/ginkgo"); err != nil {
		return err
	}

	return nil
}

func (CI) setRequiredEnvVars() error {
	if strings.Contains(jobName, "hacbs-e2e-periodic") {
		os.Setenv("E2E_TEST_SUITE_LABEL", "HACBS")
		return nil
	} else if strings.Contains(jobName, "appstudio-e2e-deployment-periodic") {
		os.Setenv("E2E_TEST_SUITE_LABEL", "!HACBS")
		return nil
	}

	if openshiftJobSpec.Refs.Repo != "e2e-tests" {

		if strings.Contains(openshiftJobSpec.Refs.Repo, "-service") {
			var envVarPrefix, imageTagSuffix, testSuiteLabel string
			sp := strings.Split(os.Getenv("COMPONENT_IMAGE"), "@")

			switch openshiftJobSpec.Refs.Repo {
			case "application-service":
				envVarPrefix = "HAS"
				imageTagSuffix = "has-image"
				testSuiteLabel = "has"
			case "build-service":
				envVarPrefix = "BUILD_SERVICE"
				imageTagSuffix = "build-service-image"
				testSuiteLabel = "build"
			case "jvm-build-service":
				envVarPrefix = "JVM_BUILD_SERVICE"
				imageTagSuffix = "jvm-build-service-image"
				testSuiteLabel = "jvm-build"
			}

			os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", envVarPrefix), sp[0])
			os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", envVarPrefix), fmt.Sprintf("redhat-appstudio-%s", imageTagSuffix))
			os.Setenv(fmt.Sprintf("%s_PR_OWNER", envVarPrefix), openshiftJobSpec.Refs.Pulls[0].Author)
			os.Setenv(fmt.Sprintf("%s_PR_SHA", envVarPrefix), openshiftJobSpec.Refs.Pulls[0].SHA)
			os.Setenv("E2E_TEST_SUITE_LABEL", testSuiteLabel)

		} else if openshiftJobSpec.Refs.Repo == "infra-deployments" {

			os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.Organization)
			os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
		}

	}

	return nil
}

func BootstrapCluster() error {
	envVars := map[string]string{}

	if os.Getenv("CI") == "true" && os.Getenv("REPO_NAME") == "e2e-tests" {
		// Some scripts in infra-deployments repo are referencing scripts/utils in e2e-tests repo
		// This env var allows to test changes introduced in "e2e-tests" repo PRs in CI
		envVars["E2E_TESTS_COMMIT_SHA"] = os.Getenv("PULL_PULL_SHA")
	}

	// TODO: https://issues.redhat.com/browse/RHDP-553 .Run here the installation script.
	//return sh.RunWith(envVars, "echo -e NOT_SUPPORTED_YET")
	return nil
}

func (CI) isPRPairingRequired() bool {
	ghBranches := &GithubBranches{}
	if err := sendHttpRequestAndParseResponse(fmt.Sprintf("https://api.github.com/repos/%s/e2e-tests/branches", pr.Author), "GET", ghBranches); err != nil {
		klog.Infof("cannot determine e2e-tests Github branches for author %s: %v. will stick with the redhat-appstudio/e2e-tests main branch for running testss", pr.Author, err)
		return false
	}

	for _, b := range ghBranches.Branches {
		if b.Name == pr.BranchName {
			return true
		}
	}

	return false
}

func (CI) sendWebhook() error {
	// AppStudio QE webhook configuration values will be used by default (if none are provided via env vars)
	const appstudioQESaltSecret = "123456789"
	const appstudioQEWebhookTargetURL = "https://smee.io/JgVqn2oYFPY1CF"

	var repoURL string

	var repoOwner = os.Getenv("REPO_OWNER")
	var repoName = os.Getenv("REPO_NAME")
	var prNumber = os.Getenv("PULL_NUMBER")
	var saltSecret = utils.GetEnv("WEBHOOK_SALT_SECRET", appstudioQESaltSecret)
	var webhookTargetURL = utils.GetEnv("WEBHOOK_TARGET_URL", appstudioQEWebhookTargetURL)

	if strings.Contains(jobName, "hacbs-e2e-periodic") {
		// TODO configure webhook channel for sending HACBS test results
		klog.Infof("not sending webhook for HACBS periodic job yet")
		return nil
	}

	if jobType == "periodic" {
		repoURL = "https://github.com/redhat-appstudio/infra-deployments"
		repoOwner = "redhat-appstudio"
		repoName = "infra-deployments"
		prNumber = "periodic"
	} else if repoName == "e2e-tests" || repoName == "infra-deployments" {
		repoURL = openshiftJobSpec.Refs.RepoLink
	} else {
		klog.Infof("sending webhook for jobType %s, jobName %s is not supported", jobType, jobName)
		return nil
	}

	path, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error when sending webhook: %+v", err)
	}

	wh := Webhook{
		Path: path,
		Repository: Repository{
			FullName:   fmt.Sprintf("%s/%s", repoOwner, repoName),
			PullNumber: prNumber,
		},
		RepositoryURL: repoURL,
	}
	resp, err := wh.CreateAndSend(saltSecret, webhookTargetURL)
	if err != nil {
		return fmt.Errorf("error sending webhook: %+v", err)
	}
	klog.Infof("webhook response: %+v", resp)

	return nil
}

// Generates test cases for Polarion(polarion.xml) from test files for AppStudio project.
func GenerateTestCasesAppStudio() error {
	return sh.RunV("ginkgo", "--v", "--dry-run", "--label-filter=$E2E_TEST_SUITE_LABEL", "./cmd", "--", "--polarion-output-file=polarion.xml", "--generate-test-cases=true")
}

func CleanUpWorkspaces(kcpEnvironment string) error {
	// Authenticate using Red Hat SSO to connect to CPS and trigger a refresh routine
	err := redHatSSOAuthentication()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	redHatSSORefresh(ctx)

	// use the specified cps environment, stable or unstable
	err = useKCPEnviroment(kcpEnvironment)
	if err != nil {
		return err
	}

	// Check oidc_login is available
	if !commandExists("kubectl-oidc_login") {
		return fmt.Errorf("kubectl-oidc_login plugin was not found")
	}

	// Initialize a kcp controller to perform actions against kcp
	kcpController, err := initKCPController()
	if err != nil {
		return err
	}

	// Switch to root workspace
	err = kcpController.SwitchToHomeWorkspace()
	if err != nil {
		return err
	}

	// List all workspaces
	workspaces, err := kcpController.ListKCPWorkspaces()
	if err != nil {
		return err
	}

	// Check creation time is older than 24 hours from execution time; if so, delete the workspace
	for _, workspace := range workspaces.Items {
		currentTime := time.Now().UTC()
		diff := currentTime.Sub(workspace.CreationTimestamp.UTC())
		if diff.Hours() >= 24 {
			klog.Infof("deleting workspace %s, it was created about %s hours ago", workspace.Name, fmt.Sprintf("%f", diff.Hours()))

			err = kcpController.DeleteKCPWorkspace(&workspace)
			if err != nil {
				klog.Infof("cannot delete workspace %s", workspace.Name)
			}
		}
	}

	return nil
}
