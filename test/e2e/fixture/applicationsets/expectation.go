package applicationsets

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/argoproj-labs/applicationset/test/e2e/fixture/applicationsets/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type state = string

const (
	failed    = "failed"
	pending   = "pending"
	succeeded = "succeeded"
)

// Expectation returns succeeded on succes condition, or pending/failed on failure, along with
// a message to describe the success/failure condition.
type Expectation func(c *Consequences) (state state, message string)

// Success asserts that the last command was successful
func Success(message string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError != nil {
			return failed, fmt.Sprintf("error: %v", c.actions.lastError)
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output did not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("no error and output contained '%s'", message)
	}
}

// Error asserts that the last command was an error with substring match
func Error(message, err string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError == nil {
			return failed, "no error"
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output does not contain '%s'", message)
		}
		if !strings.Contains(c.actions.lastError.Error(), err) {
			return failed, fmt.Sprintf("error does not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("error '%s'", message)
	}
}

// ApplicationsExist checks whether each of the 'expectedApps' exist in the namespace, and are
// equivalent to provided values.
func ApplicationsExist(expectedApps []argov1alpha1.Application) Expectation {
	return func(c *Consequences) (state, string) {

		for _, expectedApp := range expectedApps {
			foundApp := c.app(expectedApp.Name)
			if foundApp == nil {
				return pending, fmt.Sprintf("missing app '%s'", expectedApp.Name)
			}

			if !appsAreEqual(expectedApp, *foundApp) {

				diff, err := getDiff(filterFields(expectedApp), filterFields(*foundApp))
				if err != nil {
					return failed, err.Error()
				}

				return pending, fmt.Sprintf("apps are not equal: '%s', diff: %s\n", expectedApp.Name, diff)

			}

		}

		return succeeded, "all apps successfully found"
	}
}

// ApplicationsDoNotExist checks that each of the 'expectedApps' no longer exist in the namespace
func ApplicationsDoNotExist(expectedApps []argov1alpha1.Application) Expectation {
	return func(c *Consequences) (state, string) {

		for _, expectedApp := range expectedApps {
			foundApp := c.app(expectedApp.Name)
			if foundApp != nil {
				return pending, fmt.Sprintf("app '%s' should no longer exist", expectedApp.Name)
			}
		}

		return succeeded, "all apps do not exist"
	}
}

// Pod checks whether a specified condition is true for any of the pods in the namespace
func Pod(predicate func(p corev1.Pod) bool) Expectation {
	return func(c *Consequences) (state, string) {
		pods, err := pods(utils.ApplicationSetNamespace)
		if err != nil {
			return failed, err.Error()
		}
		for _, pod := range pods.Items {
			if predicate(pod) {
				return succeeded, fmt.Sprintf("pod predicate matched pod named '%s'", pod.GetName())
			}
		}
		return pending, "pod predicate does not match pods"
	}
}

func pods(namespace string) (*corev1.PodList, error) {
	fixtureClient := utils.GetE2EFixtureK8sClient()

	pods, err := fixtureClient.KubeClientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	return pods, err
}

// getDiff returns a string containing a comparison result of two applications (for test output/debug purposes)
func getDiff(orig, new argov1alpha1.Application) (string, error) {

	bytes, _, err := diff.CreateTwoWayMergePatch(orig, new, orig)
	if err != nil {
		return "", err
	}

	return string(bytes), nil

}

// filterFields returns a copy of Application, but with unnecessary (for testing) fields removed
func filterFields(input argov1alpha1.Application) argov1alpha1.Application {

	spec := input.Spec

	metaCopy := input.ObjectMeta.DeepCopy()

	output := argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      metaCopy.Labels,
			Annotations: metaCopy.Annotations,
			Name:        metaCopy.Name,
			Namespace:   metaCopy.Namespace,
			Finalizers:  metaCopy.Finalizers,
		},
		Spec: argov1alpha1.ApplicationSpec{
			Source: argov1alpha1.ApplicationSource{
				Path:           spec.Source.Path,
				RepoURL:        spec.Source.RepoURL,
				TargetRevision: spec.Source.TargetRevision,
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server:    spec.Destination.Server,
				Name:      spec.Destination.Name,
				Namespace: spec.Destination.Namespace,
			},
			Project: spec.Project,
		},
	}

	return output
}

// appsAreEqual returns true if the apps are equal, comparing only fields of interest
func appsAreEqual(one argov1alpha1.Application, two argov1alpha1.Application) bool {
	return reflect.DeepEqual(filterFields(one), filterFields(two))
}
