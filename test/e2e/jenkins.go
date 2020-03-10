package e2e

import (
	"context"
	"testing"

	"github.com/jenkinsci/kubernetes-operator/pkg/apis/jenkins/v1alpha2"
	jenkinsclient "github.com/jenkinsci/kubernetes-operator/pkg/controller/jenkins/client"
	"github.com/jenkinsci/kubernetes-operator/pkg/controller/jenkins/configuration/base/resources"
	"github.com/jenkinsci/kubernetes-operator/pkg/controller/jenkins/constants"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	userConfigurationConfigMapName = "user-config"
	userConfigurationSecretName    = "user-secret"
)

func getJenkins(t *testing.T, namespace, name string) *v1alpha2.Jenkins {
	jenkins := &v1alpha2.Jenkins{}
	namespaceName := types.NamespacedName{Namespace: namespace, Name: name}
	if err := framework.Global.Client.Get(context.TODO(), namespaceName, jenkins); err != nil {
		t.Fatal(err)
	}

	return jenkins
}

func getJenkinsMasterPod(t *testing.T, jenkins *v1alpha2.Jenkins) *corev1.Pod {
	lo := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(resources.GetJenkinsMasterPodLabels(*jenkins)).String(),
	}
	podList, err := framework.Global.KubeClient.CoreV1().Pods(jenkins.ObjectMeta.Namespace).List(lo)
	if err != nil {
		t.Fatal(err)
	}
	if len(podList.Items) != 1 {
		t.Fatalf("Jenkins pod not found, pod list: %+v", podList)
	}
	return &podList.Items[0]
}

func createJenkinsAPIClient(jenkins *v1alpha2.Jenkins, hostname string, port int) (jenkinsclient.Jenkins, error) {
	adminSecret := &corev1.Secret{}
	namespaceName := types.NamespacedName{Namespace: jenkins.Namespace, Name: resources.GetOperatorCredentialsSecretName(jenkins)}
	if err := framework.Global.Client.Get(context.TODO(), namespaceName, adminSecret); err != nil {
		return nil, err
	}

	var service corev1.Service

	err := framework.Global.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: jenkins.Namespace,
		Name:      resources.GetJenkinsHTTPServiceName(jenkins),
	}, &service)

	if err != nil {
		return nil, err
	}

	jenkinsAPIURL := jenkinsclient.JenkinsAPIConnectionSettings{
		Hostname:    hostname,
		Port:        port,
		UseNodePort: false,
	}.BuildJenkinsAPIUrl(service.Name, service.Namespace, service.Spec.Ports[0].Port, service.Spec.Ports[0].NodePort)

	return jenkinsclient.NewUserAndPasswordAuthorization(
		jenkinsAPIURL,
		string(adminSecret.Data[resources.OperatorCredentialsSecretUserNameKey]),
		string(adminSecret.Data[resources.OperatorCredentialsSecretTokenKey]),
	)
}

func createJenkinsCR(t *testing.T, name, namespace string, seedJob *[]v1alpha2.SeedJob, groovyScripts v1alpha2.GroovyScripts, casc v1alpha2.ConfigurationAsCode) *v1alpha2.Jenkins {
	var seedJobs []v1alpha2.SeedJob
	if seedJob != nil {
		seedJobs = append(seedJobs, *seedJob...)
	}

	jenkins := &v1alpha2.Jenkins{
		TypeMeta: v1alpha2.JenkinsTypeMeta(),
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha2.JenkinsSpec{
			GroovyScripts:       groovyScripts,
			ConfigurationAsCode: casc,
			Master: v1alpha2.JenkinsMaster{
				Annotations: map[string]string{"test": "label"},
				Containers: []v1alpha2.Container{
					{
						Name: resources.JenkinsMasterContainerName,
						Env: []corev1.EnvVar{
							{
								Name:  "TEST_ENV",
								Value: "test_env_value",
							},
						},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/login",
									Port:   intstr.FromString("http"),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: int32(80),
							TimeoutSeconds:      int32(4),
							FailureThreshold:    int32(10),
						},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/login",
									Port:   intstr.FromString("http"),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: int32(80),
							TimeoutSeconds:      int32(4),
							FailureThreshold:    int32(10),
						},
					},
					{
						Name:  "envoyproxy",
						Image: "envoyproxy/envoy-alpine",
					},
				},
				Plugins: []v1alpha2.Plugin{
					{Name: "audit-trail", Version: "2.4"},
					{Name: "simple-theme-plugin", Version: "0.5.1"},
					{Name: "github", Version: "1.29.4"},
				},
				NodeSelector: map[string]string{"kubernetes.io/hostname": "minikube"},
			},
			SeedJobs: seedJobs,
			Service: v1alpha2.Service{
				Type: corev1.ServiceTypeNodePort,
				Port: constants.DefaultHTTPPortInt32,
			},
		},
	}
	jenkins.Spec.Roles = []rbacv1.RoleRef{
		{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     resources.GetResourceName(jenkins),
		},
	}

	t.Logf("Jenkins CR %+v", *jenkins)
	if err := framework.Global.Client.Create(context.TODO(), jenkins, nil); err != nil {
		t.Fatal(err)
	}

	return jenkins
}

func verifyJenkinsAPIConnection(t *testing.T, jenkins *v1alpha2.Jenkins, namespace string) (jenkinsclient.Jenkins, func()) {
	podName := resources.GetJenkinsMasterPodName(*jenkins)
	port, cleanUpFunc, waitFunc, portForwardFunc, err := setupPortForwardToPod(t, namespace, podName, int(constants.DefaultHTTPPortInt32))
	if err != nil {
		t.Fatal(err)
	}
	go portForwardFunc()
	waitFunc()
	client, err := createJenkinsAPIClient(jenkins, "localhost", port)
	if err != nil {
		defer cleanUpFunc()
		t.Fatal(err)
	}

	t.Log("I can establish connection to Jenkins API")
	return client, cleanUpFunc
}

func restartJenkinsMasterPod(t *testing.T, jenkins *v1alpha2.Jenkins) {
	t.Log("Restarting Jenkins master pod")
	jenkinsPod := getJenkinsMasterPod(t, jenkins)
	err := framework.Global.Client.Delete(context.TODO(), jenkinsPod)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Jenkins master pod has been restarted")
}
