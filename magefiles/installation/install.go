package installation

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/devfile/library/pkg/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	DEFAULT_TMP_DIR                     = "tmp"
	DEFAULT_INFRA_DEPLOYMENTS_BRANCH    = "main"
	DEFAULT_INFRA_DEPLOYMENTS_GH_ORG    = "redhat-appstudio"
	DEFAULT_LOCAL_FORK_NAME             = "qe"
	DEFAULT_LOCAL_FORK_ORGANIZATION     = "redhat-appstudio-qe"
	DEFAULT_E2E_APPLICATIONS_NAMEPSPACE = "appstudio-e2e-test"
	DEFAULT_E2E_QUAY_ORG                = "redhat-appstudio-qe"
)

var (
	previewInstallArgs = []string{"preview", "--keycloak", "--toolchain"}
)

type InstallAppStudio struct {
	// Kubernetes Client to interact with Openshift Cluster
	KubernetesClient *kubeCl.CustomClient

	// TmpDirectory to store temporary files like git repos or some metadata
	TmpDirectory string

	// Directory where to clone https://github.com/redhat-appstudio/infra-deployments repo
	InfraDeploymentsCloneDir string

	// Branch to clone from https://github.com/redhat-appstudio/infra-deployments. By default will be main
	InfraDeploymentsBranch string

	// Github organization from where will be cloned
	InfraDeploymentsOrganizationName string

	// Desired fork name for testing
	LocalForkName string

	// Github organization to use for testing purposes in preview mode
	LocalGithubForkOrganization string

	// Namespace where build applications will be placed
	E2EApplicationsNamespace string

	// base64-encoded content of a docker/config.json file which contains a valid login credentials for quay.io
	QuayToken string

	// Default quay organization for repositories generated by Image-controller
	DefaultImageQuayOrg string

	// Oauth2 token for default quay organization
	DefaultImageQuayOrgOAuth2Token string
}

func NewAppStudioInstallController() (*InstallAppStudio, error) {
	cwd, _ := os.Getwd()
	k8sClient, err := kubeCl.NewAdminKubernetesClient()

	if err != nil {
		return nil, err
	}

	return &InstallAppStudio{
		KubernetesClient:                 k8sClient,
		TmpDirectory:                     DEFAULT_TMP_DIR,
		InfraDeploymentsCloneDir:         fmt.Sprintf("%s/%s/infra-deployments", cwd, DEFAULT_TMP_DIR),
		InfraDeploymentsBranch:           utils.GetEnv("INFRA_DEPLOYMENTS_BRANCH", DEFAULT_INFRA_DEPLOYMENTS_BRANCH),
		InfraDeploymentsOrganizationName: utils.GetEnv("INFRA_DEPLOYMENTS_ORG", DEFAULT_INFRA_DEPLOYMENTS_GH_ORG),
		LocalForkName:                    DEFAULT_LOCAL_FORK_NAME,
		LocalGithubForkOrganization:      utils.GetEnv("MY_GITHUB_ORG", DEFAULT_LOCAL_FORK_ORGANIZATION),
		E2EApplicationsNamespace:         utils.GetEnv("E2E_APPLICATIONS_NAMESPACE", DEFAULT_E2E_APPLICATIONS_NAMEPSPACE),
		QuayToken:                        utils.GetEnv("QUAY_TOKEN", ""),
		DefaultImageQuayOrg:              utils.GetEnv("DEFAULT_QUAY_ORG", DEFAULT_E2E_QUAY_ORG),
		DefaultImageQuayOrgOAuth2Token:   utils.GetEnv("DEFAULT_QUAY_ORG_TOKEN", ""),
	}, nil
}

func NewAppStudioInstallControllerDefaut() (*InstallAppStudio, error) {
	cwd, _ := os.Getwd()
	k8sClient, err := kubeCl.NewAdminKubernetesClient()

	if err != nil {
		return nil, err
	}

	return &InstallAppStudio{
		KubernetesClient:                 k8sClient,
		TmpDirectory:                     DEFAULT_TMP_DIR,
		InfraDeploymentsCloneDir:         fmt.Sprintf("%s/%s/infra-deployments", cwd, DEFAULT_TMP_DIR),
		InfraDeploymentsBranch:           DEFAULT_INFRA_DEPLOYMENTS_BRANCH,
		InfraDeploymentsOrganizationName: DEFAULT_INFRA_DEPLOYMENTS_GH_ORG,
		LocalForkName:                    DEFAULT_LOCAL_FORK_NAME,
		LocalGithubForkOrganization:      utils.GetEnv("MY_GITHUB_ORG", DEFAULT_LOCAL_FORK_ORGANIZATION),
		E2EApplicationsNamespace:         utils.GetEnv("E2E_APPLICATIONS_NAMESPACE", DEFAULT_E2E_APPLICATIONS_NAMEPSPACE),
		SharedSecretNamespace:            DEFAULT_SHARED_SECRETS_NAMESPACE,
		HasDefaultImageRepository:        utils.GetEnv("HAS_DEFAULT_IMAGE_REPOSITORY", fmt.Sprintf("quay.io/%s/test-images-protected", DEFAULT_E2E_QUAY_ORG)),
		QuayToken:                        utils.GetEnv("QUAY_TOKEN", ""),
	}, nil
}

// Start the appstudio installation in preview mode.
func (i *InstallAppStudio) InstallAppStudioPreviewMode() error {
	if _, err := i.cloneInfraDeployments(); err != nil {
		return err
	}
	i.setInstallationEnvironments()

	if err := utils.ExecuteCommandInASpecificDirectory("hack/bootstrap-cluster.sh", previewInstallArgs, i.InfraDeploymentsCloneDir); err != nil {
		return err
	}

	return i.createE2EQuaySecret()
}

func (i *InstallAppStudio) setInstallationEnvironments() {
	os.Setenv("MY_GITHUB_ORG", i.LocalGithubForkOrganization)
	os.Setenv("MY_GITHUB_TOKEN", utils.GetEnv("GITHUB_TOKEN", ""))
	os.Setenv("MY_GIT_FORK_REMOTE", i.LocalForkName)
	os.Setenv("E2E_APPLICATIONS_NAMESPACE", i.E2EApplicationsNamespace)
	os.Setenv("TEST_BRANCH_ID", util.GenerateRandomString(4))
	os.Setenv("QUAY_TOKEN", i.QuayToken)
	os.Setenv("IMAGE_CONTROLLER_QUAY_ORG", i.DefaultImageQuayOrg)
	os.Setenv("IMAGE_CONTROLLER_QUAY_TOKEN", i.DefaultImageQuayOrgOAuth2Token)
}

func (i *InstallAppStudio) cloneInfraDeployments() (*git.Remote, error) {
	dirInfo, err := os.Stat(i.InfraDeploymentsCloneDir)

	if !os.IsNotExist(err) && dirInfo.IsDir() {
		klog.Warningf("folder %s already exists... removing", i.InfraDeploymentsCloneDir)

		err := os.RemoveAll(i.InfraDeploymentsCloneDir)
		if err != nil {
			return nil, fmt.Errorf("error removing %s folder", i.InfraDeploymentsCloneDir)
		}
	}

	url := fmt.Sprintf("https://github.com/%s/infra-deployments", i.InfraDeploymentsOrganizationName)
	refName := fmt.Sprintf("refs/heads/%s", i.InfraDeploymentsBranch)
	klog.Infof("cloning '%s' with git ref '%s'", url, refName)
	repo, _ := git.PlainClone(i.InfraDeploymentsCloneDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.ReferenceName(refName),
		Progress:      os.Stdout,
	})

	return repo.CreateRemote(&config.RemoteConfig{Name: i.LocalForkName, URLs: []string{fmt.Sprintf("https://github.com/%s/infra-deployments.git", i.LocalGithubForkOrganization)}})
}

func (i *InstallAppStudio) MergePRInInfraDeployments() error {

	// We instance a new repository targeting the given path (the .git folder)
	//	r, err := git.PlainOpen(i.InfraDeploymentsCloneDir)
	//r, err := git.PlainOpen("tmp/infra-deployments")
	// if err != nil {
	// 	return err
	// }

	// refspec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")
	// _, err = r.CreateRemote(&config.RemoteConfig{
	// 	Name:  "jkopriva",
	// 	URLs:  []string{"https://github.com/jkopriva/infra-deployments.git"},
	// 	Fetch: []config.RefSpec{refspec},
	// })
	// if err != nil {
	// 	return err
	// }

	// Fetch using the new remote
	// err = r.Fetch(&git.FetchOptions{
	// 	RemoteName: "jkopriva",
	// })
	// if err != nil {
	// 	return err
	// }

	// Get the working directory for the repository
	// w, err := r.Worktree()
	// if err != nil {
	// 	return err
	// }

	cmd, err := exec.Command("git", "-C", "./tmp/infra-deployments", "branch").CombinedOutput()
	if err != nil {
		klog.Fatal(err)
	}
	fmt.Printf("output is %s\n", cmd)

	branch := strings.TrimSpace(strings.Replace(strings.Replace(string(cmd), " main", "", -1), "*", "", -1))

	fmt.Printf("branch is %s\n", branch)

	cmd, err = exec.Command("git", "-C", "./tmp/infra-deployments", "checkout", branch).Output()
	if err != nil {
		klog.Fatal(err)
	}
	fmt.Printf("output is %s\n", cmd)

	cmd, err = exec.Command("git", "-C", "./tmp/infra-deployments", "pull", "https://github.com/jkopriva/infra-deployments.git", "application-service", "--no-rebase", "-q").Output()
	fmt.Printf("output pull %s\n", cmd)

	if err != nil {
		klog.Fatal(err)
	}

	// cmd, err = exec.Command("git", "push", "-u", "qe").Output()
	// fmt.Printf("output push %s\n", cmd)

	// if err != nil {
	// 	klog.Fatal(err)
	// }

	// klog.Info("git pull origin")
	// err = w.Pull(&git.PullOptions{RemoteName: "jkopriva"})
	// if err != nil {
	// 	return err
	// }

	// klog.Info("git pull ec-batch-update")
	// err = w.Pull(&git.PullOptions{RemoteURL: "https://github.com/enterprise-contract/infra-deployments.git", ReferenceName: "ec-batch-update"})

	// if err != nil {
	// 	return err
	// }

	// err = r.Push(&git.PushOptions{
	// 	RemoteName: "jkopriva",
	// })

	// if err != nil {
	// 	return err
	// }

	return err
}

func checkOperatorsReady() (ready bool, err error) {
	// 	while [ -n "$(oc get applications.argoproj.io -n openshift-gitops -o jsonpath='{range .items[*]}{@.metadata.annotations.argocd\.argoproj\.io/refresh}{end}')" ]; do
	// 	sleep 5
	//   done

	//   INTERVAL=10
	//   while :; do
	// 	STATE=$(kubectl get apps -n openshift-gitops --no-headers)
	// 	NOT_DONE=$(echo "$STATE" | grep -v "Synced[[:blank:]]*Healthy" || true)
	// 	echo "$NOT_DONE"
	// 	if [ -z "$NOT_DONE" ]; then
	// 	   echo All Applications are synced and Healthy
	// 	   break
	// 	else
	// 	   UNKNOWN=$(echo "$NOT_DONE" | grep Unknown | grep -v Progressing | cut -f1 -d ' ') || :
	// 	   if [ -n "$UNKNOWN" ]; then
	// 		 for app in $UNKNOWN; do
	// 		   ERROR=$(oc get -n openshift-gitops applications.argoproj.io $app -o jsonpath='{.status.conditions}')
	// 		   if echo "$ERROR" | grep -q 'context deadline exceeded'; then
	// 			 echo Refreshing $app
	// 			 kubectl patch applications.argoproj.io $app -n openshift-gitops --type merge -p='{"metadata": {"annotations":{"argocd.argoproj.io/refresh": "soft"}}}'
	// 			 while [ -n "$(oc get applications.argoproj.io -n openshift-gitops $app -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/refresh}')" ]; do
	// 			   sleep 5
	// 			 done
	// 			 echo Refresh of $app done
	// 			 continue 2
	// 		   fi
	// 		   echo $app failed with:
	// 		   if [ -n "$ERROR" ]; then
	// 			 echo "$ERROR"
	// 		   else
	// 			 oc get -n openshift-gitops applications.argoproj.io $app -o yaml
	// 		   fi
	// 		 done
	// 		 exit 1
	// 	   fi
	// 	   echo Waiting $INTERVAL seconds for application sync
	// 	   sleep $INTERVAL
	// 	fi
	//   done

	return false, err
}

// func cmdExecutorMock(name string, _ bool, args ...string) (string, error) {
// 	var f *framework.Framework
// 	var err error
// 	var kubeadminClient *framework.ControllerHub
// 	asAdminClient, err := kubeapi.NewAdminKubernetesClient()
// 	Expect(err).ShouldNot(HaveOccurred())
// 	kubeadminClient, err = framework.InitControllerHub(asAdminClient)
// 	Expect(err).ShouldNot(HaveOccurred())
// }

// createSharedSecret make sure that redhat-appstudio-user-workload secret is created in the build-templates namespace for build purposes

// Create secret in e2e-secrets which can be copied to testing namespaces
func (i *InstallAppStudio) createE2EQuaySecret() error {
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		return fmt.Errorf("failed to obtain quay token from 'QUAY_TOKEN' env; make sure the env exists")
	}

	decodedToken, err := base64.StdEncoding.DecodeString(quayToken)
	if err != nil {
		return fmt.Errorf("failed to decode quay token. Make sure that QUAY_TOKEN env contain a base64 token")
	}

	namespace := constants.QuayRepositorySecretNamespace
	_, err = i.KubernetesClient.KubeInterface().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := i.KubernetesClient.KubeInterface().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error when creating namespace %s : %v", namespace, err)
			}
		} else {
			return fmt.Errorf("error when getting namespace %s : %v", namespace, err)
		}
	}

	secretName := constants.QuayRepositorySecretName
	secret, err := i.KubernetesClient.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := i.KubernetesClient.KubeInterface().CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: decodedToken,
				},
			}, metav1.CreateOptions{})

			if err != nil {
				return fmt.Errorf("error when creating secret %s : %v", secretName, err)
			}
		} else {
			secret.Data = map[string][]byte{
				corev1.DockerConfigJsonKey: decodedToken,
			}
			_, err = i.KubernetesClient.KubeInterface().CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("error when updating secret '%s' namespace: %v", secretName, err)
			}
		}
	}

	return nil
}
