package scope

import (
	"context"
	"fmt"

	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/giantswarm/k8sclient/v6/pkg/k8sclient"
	"github.com/giantswarm/k8sclient/v6/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	capo "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha5"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	/* #nosec G101 */
	KubeConfigSecretSuffix = "-kubeconfig"
	KubeConfigSecretKey    = "value"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	BaseDomain            string
	InfrastructureCluster *capo.OpenStackCluster
	ManagementCluster     string
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(ctx context.Context, params ClusterScopeParams) (*ClusterScope, error) {

	if params.BaseDomain == "" {
		return nil, microerror.Maskf(invalidConfigError, "failed to generate new scope from empty BaseDomain")
	}
	if params.InfrastructureCluster == nil {
		return nil, microerror.Maskf(invalidConfigError, "failed to generate new scope from nil InfrastructureCluster")
	}

	awsSession, err := session.NewSession()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &ClusterScope{
		session: awsSession,

		baseDomain:        params.BaseDomain,
		infraCluster:      params.InfrastructureCluster,
		managementCluster: params.ManagementCluster,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	k8sClient client.Client
	session   awsclient.ConfigProvider

	baseDomain        string
	infraCluster      *capo.OpenStackCluster
	managementCluster string
}

// APIEndpoint returns the Openstack infrastructure Kubernetes API endpoint.
func (s *ClusterScope) APIEndpoint() string {
	return s.infraCluster.Spec.ControlPlaneEndpoint.Host
}

// BaseDomain returns the cluster basedomain.
func (s *ClusterScope) BaseDomain() string {
	return s.baseDomain
}

// BastionIP returns the bastion IP.
func (s *ClusterScope) BastionIP() string {
	if s.infraCluster.Status.Bastion != nil {
		return s.infraCluster.Status.Bastion.FloatingIP
	}

	return ""
}

// InfrastructureCluster returns the infrastructure cluster.
func (s *ClusterScope) InfrastructureCluster() *capo.OpenStackCluster {
	return s.infraCluster
}

// ManagementCluster returns the name of the management cluster.
func (s *ClusterScope) ManagementCluster() string {
	return s.managementCluster
}

// ClusterK8sClient returns a client to interact with the cluster.
func (s *ClusterScope) ClusterK8sClient(ctx context.Context) (client.Client, error) {
	if s.k8sClient == nil {
		var err error
		s.k8sClient, err = s.getClusterK8sClient(ctx)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return s.k8sClient, nil
}

// ClusterDomain returns the cluster domain.
func (s *ClusterScope) ClusterDomain() string {
	return fmt.Sprintf("%s.%s", s.Name(), s.baseDomain)
}

// Name returns the Openstack cluster name.
func (s *ClusterScope) Name() string {
	return s.infraCluster.Labels[capi.ClusterLabelName]
}

// Session returns the AWS SDK session. Used for creating cluster client.
func (s *ClusterScope) Session() awsclient.ConfigProvider {
	return s.session
}

func (s *ClusterScope) getClusterK8sClient(ctx context.Context) (client.Client, error) {
	newLogger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	kubeconfig, err := s.getClusterKubeConfig(ctx, newLogger)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	config := k8srestconfig.Config{
		Logger:     newLogger,
		KubeConfig: kubeconfig,
	}

	return getK8sClient(config, newLogger)
}

func (s *ClusterScope) getClusterKubeConfig(ctx context.Context, logger micrologger.Logger) (string, error) {
	config := k8srestconfig.Config{
		Logger:    logger,
		InCluster: true,
	}

	k8sClient, err := getK8sClient(config, logger)
	if err != nil {
		return "", microerror.Mask(err)
	}

	var secret corev1.Secret

	o := client.ObjectKey{
		Name:      fmt.Sprintf("%s%s", s.Name(), KubeConfigSecretSuffix),
		Namespace: s.infraCluster.Namespace,
	}

	if err := k8sClient.Get(ctx, o, &secret); err != nil {
		return "", microerror.Mask(err)
	}

	return string(secret.Data[KubeConfigSecretKey]), nil
}

func getK8sClient(config k8srestconfig.Config, logger micrologger.Logger) (client.Client, error) {
	var restConfig *rest.Config
	var err error
	{
		restConfig, err = k8srestconfig.New(config)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var ctrlClient client.Client
	{
		c := k8sclient.ClientsConfig{
			Logger:     logger,
			RestConfig: restConfig,
		}

		k8sClients, err := k8sclient.NewClients(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		ctrlClient = k8sClients.CtrlClient()
	}

	return ctrlClient, nil
}
