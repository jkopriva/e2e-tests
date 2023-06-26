package setup

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	ProvisionedUser          = "mig-prov"
	DeactivatedUser          = "mig-deact"
	BannedUser               = "mig-banned"
	AppStudioProvisionedUser = "mig-appst"

	ProvisionedAppStudioSpace = "mig-appst-space"

	DEFAULT_USER_PRIVATE_REPOS = "has-e2e-private"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("users"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var testNamespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(DEFAULT_USER_PRIVATE_REPOS)
		Expect(err).NotTo(HaveOccurred())

		testNamespace = fw.UserNamespace
		Expect(testNamespace).NotTo(BeEmpty())
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)

		// createAPIProxyClient := func(userToken, proxyURL string) *crclient.Client {
		// 	APIProxyClient, err := client.CreateAPIProxyClient(userToken, proxyURL)
		// 	Expect(err).NotTo(HaveOccurred())
		// 	client := APIProxyClient.KubeRest()
		// 	return &client
		// }
	})

	It("creates AppStudioProvisionedSpace", func() {
		// hostAwait := r.Awaitilities.Host()
		// space := testsupport.NewSpace(t, r.Awaitilities, testsupport.WithName(ProvisionedAppStudioSpace), testsupport.WithTierName("appstudio"), testsupport.WithTargetCluster(targetCluster.ClusterName))
		// err := hostAwait.Client.Create(context.TODO(), space)
		// Expect(err).NotTo(HaveOccurred())

		// _, _, binding := testsupport.CreateMurWithAdminSpaceBindingForSpace(t, r.Awaitilities, space, r.WithCleanup)

		// tier, err := hostAwait.WaitForNSTemplateTier(t, tierName)
		// Expect(err).NotTo(HaveOccurred())

		// _, err = targetCluster.WaitForNSTmplSet(t, space.Name,
		// 	wait.UntilNSTemplateSetHasConditions(wait.Provisioned()),
		// 	wait.UntilNSTemplateSetHasSpaceRoles(
		// 		wait.SpaceRole(tier.Spec.SpaceRoles[binding.Spec.SpaceRole].TemplateRef, binding.Spec.MasterUserRecord)))
		// Expect(err).NotTo(HaveOccurred())

		// _, err = hostAwait.WaitForSpace(t, space.Name,
		// 	wait.UntilSpaceHasConditions(wait.Provisioned()))
		// if err != nil {
		// 	klog.Errorf("%s", err)
		// }
		// // if r.WithCleanup {
		// // 	cleanup.AddCleanTasks(t, r.Awaitilities.Host().Client, space)
		// // }

		// proxyAuthInfo, err := sandboxController.ReconcileUserCreation(userName)
		// Expect(err).NotTo(HaveOccurred())

	})

	It("creates AppStudioProvisionedUser", func() {
		_, err := fw.SandboxController.RegisterSandboxUser(AppStudioProvisionedUser)
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates AppStudio Deactivated User", func() {
		// compliantUsername, err := fw.SandboxController.RegisterSandboxUser(DeactivatedUser)
		// Expect(err).NotTo(HaveOccurred())

		// // deactivate the UserSignup
		// userSignup, err := hostAwait.UpdateUserSignup(t, compliantUsername,
		// 	func(us *toolchainv1alpha1.UserSignup) {
		// 		states.SetDeactivated(us, true)
		// 	})
		// Expect(err).NotTo(HaveOccurred())
		// klog.Info("user signup '%s' set to deactivated", userSignup.Name)

		// // verify that MUR is deleted
		// err = hostAwait.WaitUntilMasterUserRecordAndSpaceBindingsDeleted(t, userSignup.Status.CompliantUsername) // TODO wait for space deletion too after Space migration is done
		// Expect(err).NotTo(HaveOccurred())
	})

	It("creates AppStudio Banned User", func() {
		_, err := fw.SandboxController.RegisterBannedSandboxUser(AppStudioProvisionedUser)
		Expect(err).NotTo(HaveOccurred())

		// // Create the BannedUser
		// bannedUser := testsupport.NewBannedUser(hostAwait, userSignup.Annotations[toolchainv1alpha1.UserSignupUserEmailAnnotationKey])
		// err = hostAwait.Client.Create(context.TODO(), bannedUser)
		// Expect(err).NotTo(HaveOccurred())

		// klog.Info("AppStudioBannedUser '%s' created", bannedUser.Spec.Email)

		// // Confirm the user is banned
		// _, err = hostAwait.WithRetryOptions(wait.TimeoutOption(time.Second*15)).WaitForUserSignup(t, userSignup.Name,
		// 	wait.ContainsCondition(wait.Banned()[0]))
		Expect(err).NotTo(HaveOccurred())
	})

})
