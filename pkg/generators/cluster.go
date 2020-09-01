package generators

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj-labs/applicationset/api/v1alpha1"
)

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
)

var _ Generator = (*ClusterGenerator)(nil)

// ClusterGenerator generates Applications for some or all clusters registered with ArgoCD.
type ClusterGenerator struct {
	client.Client
}


func NewClusterGenerator(c client.Client) Generator {
	g := &ClusterGenerator{
		Client: c,
	}
	return g
}

func (g *ClusterGenerator) GenerateParams(
	appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) ([]map[string]string, error) {

	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError{}
	}

	if appSetGenerator.Clusters == nil {
		return nil, nil
	}

	// List all Clusters:
	clusterSecretList := &corev1.SecretList{}
	secretLabels := map[string]string{
		ArgoCDSecretTypeLabel: ArgoCDSecretTypeCluster,
	}
	for k, v := range appSetGenerator.Clusters.Selector.MatchLabels {
		secretLabels[k] = v
	}
	if err := g.Client.List(context.Background(), clusterSecretList, client.MatchingLabels(secretLabels)); err != nil {
		return nil, err
	}
	log.Debug("clusters matching labels", "count", len(clusterSecretList.Items))

	res := make([]map[string]string, len(clusterSecretList.Items))
	for i, cluster := range clusterSecretList.Items {
		params := make(map[string]string, len(cluster.ObjectMeta.Labels) + 2)
		params["name"] = cluster.Name
		params["server"] = string(cluster.Data["server"])
		for key, value := range cluster.ObjectMeta.Labels {
			params[fmt.Sprintf("metadata.labels.%s", key)] = value
		}
		log.WithField("cluster", cluster.Name).Info("matched cluster secret")

		res[i] = params
	}

	return res, nil
}
