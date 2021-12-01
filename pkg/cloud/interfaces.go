package cloud

import (
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	infrav1 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Session represents an AWS session
type Session interface {
	Session() awsclient.ConfigProvider
}

// ClusterObject represents a AWS cluster object
type ClusterObject interface {
	conditions.Setter
}

// ClusterScoper is the interface for a cluster scope
type ClusterScoper interface {
	logr.Logger
	Session

	// APIEndpoint returns the AWS infrastructure Kubernetes LoadBalancer API endpoint.
	// e.g. apiserver-x.eu-central-1.elb.amazonaws.com
	APIEndpoint() string
	// BaseDomain returns the base domain.
	BaseDomain() string
	// Cluster returns the AWS infrastructure cluster object.
	Cluster() *infrav1.OpenStackCluster
	// Name returns the CAPI cluster name.
	Name() string
}
