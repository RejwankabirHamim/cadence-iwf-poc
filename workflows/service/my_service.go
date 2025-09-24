package service

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/RejwankabirHamim/cadence-iwf-poc/pkg/common"
)

const (
	RetryInterval     = 10 * time.Second
	RetryTimeout      = 30 * time.Minute
	CAPIRunnerJobName = "capi-runner"
)

type MyService interface {
	CreateNamespace(ctx context.Context, nsname string) error
	CreateJob(ctx context.Context, op common.KubeVirtCreateOperation, namespace string) error
	IsClusterOperationSuccessful(ctx context.Context, namespace string) error
	SyncCredential()
	CleanupNamespace()
}

type myServiceImpl struct {
	k8sClient client.Client
}

func (m *myServiceImpl) CreateNamespace(ctx context.Context, nsname string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsname,
		},
	}
	return m.k8sClient.Create(ctx, ns, &client.CreateOptions{})
}

func (m *myServiceImpl) CreateJob(ctx context.Context, op common.KubeVirtCreateOperation, namespace string) error {
	scriptSecretName := namespace

	err := op.CreateScriptSecret(ctx, m.k8sClient, scriptSecretName)
	if err != nil {
		return err
	}

	imgName, err := op.GetBaseImage(ctx, m.k8sClient)
	if err != nil {
		return err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-runner",
			Namespace: namespace,
			Labels: map[string]string{
				"cluster-name": op.CAPIConfig.ClusterName,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "capi-script-runner",
							Image: imgName,
							Command: []string{
								"/etc/capi-script/script.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "script",
									ReadOnly:  true,
									MountPath: "/etc/capi-script",
								},
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:    ptr.To(int64(1000)),
								RunAsGroup:   ptr.To(int64(1000)),
								RunAsNonRoot: ptr.To(true),
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "script",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  scriptSecretName,
									DefaultMode: ptr.To(int32(0o755)),
								},
							},
						},
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			BackoffLimit: ptr.To(int32(0)),
		},
	}

	return m.k8sClient.Create(ctx, job, &client.CreateOptions{})
}

func (m *myServiceImpl) IsClusterOperationSuccessful(ctx context.Context, namespace string) error {
	job := &batchv1.Job{}
	return wait.PollUntilContextTimeout(ctx, RetryInterval, RetryTimeout, true, func(ctx context.Context) (bool, error) {
		err := m.k8sClient.Get(ctx, types.NamespacedName{
			Name:      CAPIRunnerJobName,
			Namespace: namespace,
		}, job)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				return false, err
			} else {
				return false, nil
			}
		}
		if job.Status.Succeeded > 0 {
			return true, nil
		}
		if job.Status.Failed > 0 {
			return false, fmt.Errorf("failed to perform cluster operation")
		}
		return false, nil
	})
}

func (m *myServiceImpl) SyncCredential()   {}
func (m *myServiceImpl) CleanupNamespace() {}

func NewMyService(k8sClient client.Client) MyService {
	return &myServiceImpl{k8sClient: k8sClient}
}
