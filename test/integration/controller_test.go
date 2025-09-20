package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/controllers"
)

// These tests use Ginkgo 2.x
var _ = Describe("JiraCDC Controller", func() {
	Context("When reconciling a JiraCDC resource", func() {
		const (
			JiraCDCName      = "test-jiracdc"
			JiraCDCNamespace = "default"
			timeout          = time.Second * 10
			interval         = time.Millisecond * 250
		)

		ctx := context.Background()

		jiracdc := &jiradcdv1.JiraCDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      JiraCDCName,
				Namespace: JiraCDCNamespace,
			},
			Spec: jiradcdv1.JiraCDCSpec{
				JiraInstance: jiradcdv1.JiraInstanceConfig{
					BaseURL:           "https://test.atlassian.net",
					CredentialsSecret: "jira-creds",
				},
				SyncTarget: jiradcdv1.SyncTargetConfig{
					Type:       "project",
					ProjectKey: "TEST",
				},
				GitRepository: jiradcdv1.GitRepositoryConfig{
					URL:               "git@github.com:test/repo.git",
					CredentialsSecret: "git-creds",
					Branch:            "main",
				},
			},
		}

		BeforeEach(func() {
			// This will fail until JiraCDC types are implemented
			By("creating the custom resource for the Kind JiraCDC")
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), jiracdc)
			if err != nil && client.IgnoreNotFound(err) == nil {
				Expect(k8sClient.Create(ctx, jiracdc)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Cleanup the test resource after each test
			By("Cleanup the specific resource instance JiraCDC")
			resource := &jiradcdv1.JiraCDC{}
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance JiraCDC")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			// This will fail until the controller is implemented
			controllerReconciler := &controllers.JiraCDCReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(jiracdc),
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &jiradcdv1.JiraCDC{}
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), found)
			}, timeout, interval).Should(Succeed())
		})

		It("Should update the JiraCDC status after reconciliation", func() {
			By("Checking the JiraCDC instance status")
			// This will fail until status updates are implemented
			Eventually(func() bool {
				found := &jiradcdv1.JiraCDC{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), found)
				if err != nil {
					return false
				}
				// Check that status has been updated
				return found.Status.Phase != ""
			}, timeout, interval).Should(BeTrue())
		})

		It("Should create API operand deployment", func() {
			By("Checking if API operand deployment was created")
			// This will fail until operand management is implemented
			Eventually(func() bool {
				// Look for the API deployment created by the controller
				// This is a placeholder - actual implementation will check for real deployments
				found := &jiradcdv1.JiraCDC{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), found)
				if err != nil {
					return false
				}
				// Check operand status
				return found.Status.OperandStatus.API.Ready
			}, timeout, interval).Should(BeTrue())
		})

		It("Should create UI operand deployment", func() {
			By("Checking if UI operand deployment was created")
			// This will fail until operand management is implemented
			Eventually(func() bool {
				found := &jiradcdv1.JiraCDC{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), found)
				if err != nil {
					return false
				}
				// Check operand status
				return found.Status.OperandStatus.UI.Ready
			}, timeout, interval).Should(BeTrue())
		})

		It("Should handle validation errors gracefully", func() {
			By("Creating a JiraCDC with invalid configuration")
			invalidJiraCDC := &jiradcdv1.JiraCDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-jiracdc",
					Namespace: JiraCDCNamespace,
				},
				Spec: jiradcdv1.JiraCDCSpec{
					// Missing required fields
					SyncTarget: jiradcdv1.SyncTargetConfig{
						Type: "invalid-type",
					},
				},
			}

			// This should fail validation
			err := k8sClient.Create(ctx, invalidJiraCDC)
			Expect(err).To(HaveOccurred())
		})

		It("Should reconcile spec changes", func() {
			By("Updating the JiraCDC spec")
			found := &jiradcdv1.JiraCDC{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), found)).To(Succeed())

			// Update sync configuration
			found.Spec.SyncConfig.Interval = "10m"
			found.Spec.SyncConfig.ActiveIssuesOnly = true

			Expect(k8sClient.Update(ctx, found)).To(Succeed())

			By("Checking that reconciliation updates operands")
			Eventually(func() bool {
				updated := &jiradcdv1.JiraCDC{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(jiracdc), updated)
				if err != nil {
					return false
				}
				// Controller should update status to reflect new configuration
				return updated.Status.LastReconcileTime != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file to be used in tests
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// This will fail until JiraCDC types are implemented and CRDs are generated
	err = jiradcdv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Start the controller manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	// This will fail until the controller is implemented
	err = (&controllers.JiraCDCReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})