package create

import (
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	utils "github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
)

func CreateAppStudioProvisionedSpace(fw *framework.Framework) {
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

}

func CreateAppStudioProvisionedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterSandboxUser(utils.AppStudioProvisionedUser)
	Expect(err).NotTo(HaveOccurred())
}

func CreateAppStudioDeactivatedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterDeactivatedSandboxUser(utils.DeactivatedUser)
	Expect(err).NotTo(HaveOccurred())
}

func CreateAppStudioBannedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterBannedSandboxUser(utils.BannedUser)
	Expect(err).NotTo(HaveOccurred())
}
