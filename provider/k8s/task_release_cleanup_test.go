package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	convoxv1 "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/convox/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockProviderForReleaseCleaner is a mock implementation of providerForReleaseCleaner
type MockProviderForReleaseCleaner struct {
	mock.Mock
}

func (m *MockProviderForReleaseCleaner) AppList() (structs.Apps, error) {
	args := m.Called()
	return args.Get(0).(structs.Apps), args.Error(1)
}

func (m *MockProviderForReleaseCleaner) AppNamespace(appName string) string {
	args := m.Called(appName)
	return args.String(0)
}

// MockEngine is a mock implementation of Engine for testing
type MockEngine struct {
	mock.Mock
}

func (m *MockEngine) RepositoryImagesBatchDelete(app string, builds []string) error {
	args := m.Called(app, builds)
	return args.Error(0)
}

// Add stub implementations for all Engine interface methods
func (m *MockEngine) AppIdles(app string) (bool, error) {
	return false, nil
}

func (m *MockEngine) AppParameters() map[string]string {
	return nil
}

func (m *MockEngine) GPUIntanceList(instanceTypes []string) ([]string, error) {
	return nil, nil
}

func (m *MockEngine) Heartbeat() (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockEngine) IngressAnnotations(certDuration string) (map[string]string, error) {
	return nil, nil
}

func (m *MockEngine) IngressClass() string {
	return ""
}

func (m *MockEngine) IngressInternalClass() string {
	return ""
}

func (m *MockEngine) Log(app, stream string, ts time.Time, message string) error {
	return nil
}

func (m *MockEngine) ManifestValidate(manifest *manifest.Manifest) error {
	args := m.Called(manifest)
	return args.Error(0)
}

func (m *MockEngine) RegistryAuth(host, username, password string) (string, string, error) {
	return "", "", nil
}

func (m *MockEngine) RepositoryAuth(app string) (string, string, error) {
	return "", "", nil
}

func (m *MockEngine) RepositoryHost(app string) (string, bool, error) {
	return "", false, nil
}

func (m *MockEngine) RepositoryPrefix() string {
	return ""
}

func (m *MockEngine) ResolverHost() (string, error) {
	return "", nil
}

func (m *MockEngine) ServiceHost(app string, s *manifest.Service) string {
	args := m.Called(app, s)
	return args.String(0)
}

func (m *MockEngine) SystemHost() string {
	return ""
}

func (m *MockEngine) SystemStatus() (string, error) {
	return "", nil
}

// Helper function to create a releaseCleaner for testing
func createReleaseCleaner(
	provider providerForReleaseCleaner,
	engine Engine,
	convox *cvfake.Clientset,
	cluster *fake.Clientset,
	systemNamespace string,
	releasesToRetainAfterActive int,
) *releaseCleaner {
	log := logger.New("test")

	return &releaseCleaner{
		provider:                    provider,
		engine:                      engine,
		convox:                      convox,
		logger:                      log,
		cluster:                     cluster,
		systemNamespace:             systemNamespace,
		ctx:                         context.Background(),
		releasesToRetainAfterActive: releasesToRetainAfterActive,
	}
}

// Helper functions for creating test data
func createTestRelease(name, ns, build, created string) convoxv1.Release {
	return convoxv1.Release{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: convoxv1.ReleaseSpec{
			Build:   build,
			Created: created,
		},
	}
}

func createTestBuild(name, ns, started string) convoxv1.Build {
	return convoxv1.Build{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: convoxv1.BuildSpec{
			Started: started,
		},
	}
}

// Create a namespace for testing
func createNamespace(cluster *fake.Clientset, name string, annotations map[string]string) error {
	ns := &corev1.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
	_, err := cluster.CoreV1().Namespaces().Create(context.Background(), ns, am.CreateOptions{})
	return err
}

// Test for the waitUntilScheduledForCleanup method
func TestWaitUntilScheduledForCleanup(t *testing.T) {
	systemNamespace := "test-namespace"
	now := time.Now().UTC()
	annotationTS := now.Add(-2 * time.Hour).Format(time.RFC3339)
	configMapTS := now.Add(-3 * time.Hour).Format(time.RFC3339)

	testCases := []struct {
		name               string
		annotation         string
		configMapTimestamp string
		expectTimestamp    string
	}{
		{
			name:               "Creates configmap when missing",
			annotation:         "",
			configMapTimestamp: "",
			expectTimestamp:    "",
		},
		{
			name:               "Copies annotation when configmap missing",
			annotation:         annotationTS,
			configMapTimestamp: "",
			expectTimestamp:    annotationTS,
		},
		{
			name:               "Keeps existing configmap timestamp",
			annotation:         "",
			configMapTimestamp: configMapTS,
			expectTimestamp:    configMapTS,
		},
		{
			name:               "Handles invalid timestamp",
			annotation:         "",
			configMapTimestamp: "invalid-date-format",
			expectTimestamp:    "invalid-date-format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock objects for each test case
			provider := &MockProviderForReleaseCleaner{}
			engine := &MockEngine{}
			convox := cvfake.NewSimpleClientset()
			cluster := fake.NewSimpleClientset()

			// Create namespace with annotation
			ns := &corev1.Namespace{
				ObjectMeta: am.ObjectMeta{
					Name: systemNamespace,
				},
			}

			if tc.annotation != "" {
				ns.Annotations = map[string]string{
					cleanupAnnotationKey: tc.annotation,
				}
			}

			_, err := cluster.CoreV1().Namespaces().Create(context.Background(), ns, am.CreateOptions{})
			require.NoError(t, err)

			if tc.configMapTimestamp != "" {
				cm := &corev1.ConfigMap{
					ObjectMeta: am.ObjectMeta{
						Name:      cleanupConfigMapName,
						Namespace: systemNamespace,
					},
					Data: map[string]string{
						cleanupTimestampKey: tc.configMapTimestamp,
					},
				}
				_, err := cluster.CoreV1().ConfigMaps(systemNamespace).Create(context.Background(), cm, am.CreateOptions{})
				require.NoError(t, err)
			}

			// Create the cleaner
			cleaner := createReleaseCleaner(provider, engine, convox, cluster, systemNamespace, 3)

			// Override the context with a canceled one to avoid long sleeps in tests
			cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			cleaner.ctx = cancelCtx

			// Call the method
			err = cleaner.waitUntilScheduledForCleanup()

			assert.NoError(t, err)

			cm, err := cluster.CoreV1().ConfigMaps(systemNamespace).Get(context.Background(), cleanupConfigMapName, am.GetOptions{})
			require.NoError(t, err)

			got := cm.Data[cleanupTimestampKey]
			if tc.expectTimestamp == "" {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tc.expectTimestamp, got)
			}
		})
	}
}

func TestUpdateCleanupTimestamp(t *testing.T) {
	provider := &MockProviderForReleaseCleaner{}
	engine := &MockEngine{}
	convox := cvfake.NewSimpleClientset()
	cluster := fake.NewSimpleClientset()

	systemNamespace := "test-namespace"
	_, err := cluster.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: systemNamespace,
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	cleaner := createReleaseCleaner(provider, engine, convox, cluster, systemNamespace, 3)

	ts := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, cleaner.updateCleanupTimestamp(ts))

	cm, err := cluster.CoreV1().ConfigMaps(systemNamespace).Get(context.Background(), cleanupConfigMapName, am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, ts.Format(time.RFC3339), cm.Data[cleanupTimestampKey])

	next := ts.Add(30 * time.Minute)
	require.NoError(t, cleaner.updateCleanupTimestamp(next))

	cm, err = cluster.CoreV1().ConfigMaps(systemNamespace).Get(context.Background(), cleanupConfigMapName, am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, next.Format(time.RFC3339), cm.Data[cleanupTimestampKey])
}

// Test for appReleaseAndBuildCleanup method
func TestAppReleaseAndBuildCleanup(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                        string
		app                         structs.App
		releases                    []convoxv1.Release
		builds                      []convoxv1.Build
		releasesToRetainAfterActive int
		deleteImagesError           error
		expectError                 bool
		expectedDeletedBuilds       []string
		expectedDeletedReleases     []string
	}{
		{
			name: "No active release found",
			app: structs.App{
				Name:    "app1",
				Release: "release-not-found",
			},
			releases: []convoxv1.Release{
				createTestRelease("release1", "app1-namespace", "build1", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
				createTestRelease("release2", "app1-namespace", "build2", time.Now().Add(-2*time.Hour).Format(common.SortableTime)),
			},
			builds: []convoxv1.Build{
				createTestBuild("build1", "app1-namespace", time.Now().Add(-4*time.Hour).Format(common.SortableTime)),
				createTestBuild("build2", "app1-namespace", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
			},
			releasesToRetainAfterActive: 3,
			expectError:                 false,
		},
		{
			name: "empty releases list",
			app: structs.App{
				Name:    "app1",
				Release: "release1",
			},
			releasesToRetainAfterActive: 3,
			expectError:                 false,
		},
		{
			name: "empty builds list",
			app: structs.App{
				Name:    "app1",
				Release: "release1",
			},
			releases: []convoxv1.Release{
				createTestRelease("release1", "app1-namespace", "build1", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
			},
			releasesToRetainAfterActive: 3,
			expectError:                 false,
		},
		{
			name: "Normal cleanup",
			app: structs.App{
				Name:    "app1",
				Release: "release1",
			},
			releases: []convoxv1.Release{
				createTestRelease("release1", "app1-namespace", "build1", time.Now().Add(-1*time.Hour).Format(common.SortableTime)),
				createTestRelease("release2", "app1-namespace", "build2", time.Now().Add(-2*time.Hour).Format(common.SortableTime)),
				createTestRelease("release3", "app1-namespace", "build3", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
				createTestRelease("release4", "app1-namespace", "build4", time.Now().Add(-4*time.Hour).Format(common.SortableTime)),
				createTestRelease("release5", "app1-namespace", "build5", time.Now().Add(-5*time.Hour).Format(common.SortableTime)),
			},
			builds: []convoxv1.Build{
				createTestBuild("build1", "app1-namespace", time.Now().Add(-7*time.Hour).Format(common.SortableTime)),
				createTestBuild("build2", "app1-namespace", time.Now().Add(-6*time.Hour).Format(common.SortableTime)),
				createTestBuild("build3", "app1-namespace", time.Now().Add(-5*time.Hour).Format(common.SortableTime)),
				createTestBuild("build4", "app1-namespace", time.Now().Add(-4*time.Hour).Format(common.SortableTime)),
				createTestBuild("build5", "app1-namespace", time.Now().Add(-3*time.Hour).Format(common.SortableTime)), // same time as last old release to keep
				createTestBuild("build6", "app1-namespace", time.Now().Add(-8*time.Hour).Format(common.SortableTime)),
				createTestBuild("build7", "app1-namespace", time.Now().Add(-9*time.Hour).Format(common.SortableTime)),
				createTestBuild("build8", "app1-namespace", time.Now().Add(1*time.Hour).Format(common.SortableTime)),
			},
			releasesToRetainAfterActive: 2, // Keep release1, release2, release3
			expectError:                 false,
			expectedDeletedBuilds:       []string{"build4", "build6", "build7"},
			expectedDeletedReleases:     []string{"release4", "release5"},
		},
		{
			name: "Normal cleanup",
			app: structs.App{
				Name:    "app1",
				Release: "release2",
			},
			releases: []convoxv1.Release{
				createTestRelease("release1", "app1-namespace", "build1", time.Now().Add(-1*time.Hour).Format(common.SortableTime)),
				createTestRelease("release2", "app1-namespace", "build2", time.Now().Add(-2*time.Hour).Format(common.SortableTime)),
				createTestRelease("release3", "app1-namespace", "build3", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
				createTestRelease("release4", "app1-namespace", "build4", time.Now().Add(-4*time.Hour).Format(common.SortableTime)),
				createTestRelease("release5", "app1-namespace", "build5", time.Now().Add(-5*time.Hour).Format(common.SortableTime)),
			},
			builds: []convoxv1.Build{
				createTestBuild("build1", "app1-namespace", time.Now().Add(-7*time.Hour).Format(common.SortableTime)),
				createTestBuild("build2", "app1-namespace", time.Now().Add(-6*time.Hour).Format(common.SortableTime)),
				createTestBuild("build3", "app1-namespace", time.Now().Add(-5*time.Hour).Format(common.SortableTime)),
				createTestBuild("build4", "app1-namespace", time.Now().Add(-4*time.Hour).Format(common.SortableTime)), // same time as last old release to keep
				createTestBuild("build5", "app1-namespace", time.Now().Add(-3*time.Hour).Format(common.SortableTime)),
				createTestBuild("build6", "app1-namespace", time.Now().Add(-8*time.Hour).Format(common.SortableTime)),
				createTestBuild("build7", "app1-namespace", time.Now().Add(-9*time.Hour).Format(common.SortableTime)),
				createTestBuild("build8", "app1-namespace", time.Now().Add(1*time.Hour).Format(common.SortableTime)),
			},
			releasesToRetainAfterActive: 2, // Keep release1, release2, release3
			expectError:                 false,
			expectedDeletedBuilds:       []string{"build6", "build7"},
			expectedDeletedReleases:     []string{"release5"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock objects
			mockProvider := &MockProviderForReleaseCleaner{}
			mockEngine := &MockEngine{}
			convox := cvfake.NewSimpleClientset()
			cluster := fake.NewSimpleClientset()

			// Mock the AppNamespace method
			mockProvider.On("AppNamespace", tc.app.Name).Return(tc.app.Name + "-namespace")

			// Setup test data
			// In a real test, you would actually create these objects in the fake client
			// But since we'll mock the List calls, we don't need to do that here
			for _, r := range tc.releases {
				convox.ConvoxV1().Releases(r.Namespace).Create(&r)
			}

			for _, b := range tc.builds {
				convox.ConvoxV1().Builds(b.Namespace).Create(&b)
			}

			// Create the cleaner
			cleaner := createReleaseCleaner(mockProvider, mockEngine, convox, cluster, "test-namespace", tc.releasesToRetainAfterActive)

			// Call the method
			err := cleaner.appReleaseAndBuildCleanup(&tc.app)

			// Verify results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockProvider.AssertExpectations(t)
			mockEngine.AssertExpectations(t)

			for _, b := range tc.builds {
				_, err := convox.ConvoxV1().Builds(b.Namespace).Get(b.Name, am.GetOptions{})
				if common.ContainsInStringSlice(tc.expectedDeletedBuilds, b.Name) {
					assert.Error(t, err, "Expected build %s to be deleted", b.Name)
				} else {
					assert.NoError(t, err, "Expected build %s to exist", b.Name)
				}
			}

			for _, r := range tc.releases {
				_, err := convox.ConvoxV1().Releases(r.Namespace).Get(r.Name, am.GetOptions{})
				if common.ContainsInStringSlice(tc.expectedDeletedReleases, r.Name) {
					assert.Error(t, err, "Expected release %s to be deleted", r.Name)
				} else {
					assert.NoError(t, err, "Expected release %s to exist", r.Name)
				}
			}
		})
	}
}
