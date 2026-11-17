package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The review kinds: questions to the API server, made by creating one and
// read from its answer; none is stored, so none can be listed.
//
//	SelfSubjectReview          who am I?              kubectl auth whoami
//	SelfSubjectAccessReview    may I do this?         kubectl auth can-i
//	SelfSubjectRulesReview     what may I do here?    kubectl auth can-i --list
//	SubjectAccessReview        may they do this?      kubectl auth can-i --as
//	LocalSubjectAccessReview   may they, here?
//	TokenReview                whose token is this?

// WhoAmI asks who the client is, like `kubectl auth whoami`: the user
// name, its groups and extra, as the API server sees them.
func (c *Client) WhoAmI(ctx context.Context) (user string, groups []string, err error) {
	review, err := c.clientset.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}
	info := review.Status.UserInfo
	groups = append([]string{}, info.Groups...)
	sort.Strings(groups)
	return info.Username, groups, nil
}

// userText is a user and its groups, one per line.
func userText(user string, groups []string) string {
	return fmt.Sprintf("Username: %s\nGroups:\n  %s", user, strings.Join(groups, "\n  "))
}

// WhoAmIText is WhoAmI as a page of text.
func (c *Client) WhoAmIText(ctx context.Context) (string, error) {
	user, groups, err := c.WhoAmI(ctx)
	return userText(user, groups), err
}

// accessAttributes is the question "may VERB RESOURCE in NAMESPACE": the
// resource as typed (see resourceGroups), the namespace "" for all, or
// for a resource not in one.
func (c *Client) accessAttributes(ctx context.Context, verb, resource, namespace string) (*authorizationv1.ResourceAttributes, error) {
	if verb == "" || resource == "" {
		return nil, fmt.Errorf("type a verb and a resource, e.g. create deployments")
	}
	groupOf, err := c.resourceGroups(ctx)
	if err != nil {
		return nil, err
	}
	name, group := "*", "*"
	if resource != "*" {
		if name, group, err = groupOf(resource); err != nil {
			return nil, err
		}
	}
	return &authorizationv1.ResourceAttributes{Verb: verb, Group: group, Resource: name, Namespace: namespace}, nil
}

// answerText is an access review's answer: yes or no, and why if the
// server says.
func answerText(status authorizationv1.SubjectAccessReviewStatus) string {
	answer := "no"
	if status.Allowed {
		answer = "yes"
	}
	if status.Reason != "" {
		answer += "\n\n" + status.Reason
	}
	if status.EvaluationError != "" {
		answer += "\n\n(" + status.EvaluationError + ")"
	}
	return answer
}

// CanI asks whether the client may do verb to resource in namespace, like
// `kubectl auth can-i VERB RESOURCE -n NAMESPACE`: "yes" or "no".
func (c *Client) CanI(ctx context.Context, verb, resource, namespace string) (string, error) {
	attrs, err := c.accessAttributes(ctx, verb, resource, namespace)
	if err != nil {
		return "", err
	}
	review := &authorizationv1.SelfSubjectAccessReview{Spec: authorizationv1.SelfSubjectAccessReviewSpec{ResourceAttributes: attrs}}
	answer, err := c.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return answerText(answer.Status), nil
}

// MyRules lists what the client may do in namespace, like `kubectl auth
// can-i --list -n NAMESPACE`: a line per rule, its resources then its
// verbs, lined up. The server may not know them all (incomplete, e.g. with
// a webhook authorizer); it says so on the last line.
func (c *Client) MyRules(ctx context.Context, namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}
	review := &authorizationv1.SelfSubjectRulesReview{Spec: authorizationv1.SelfSubjectRulesReviewSpec{Namespace: namespace}}
	answer, err := c.clientset.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	rows := [][]string{{"RESOURCES", "NAMES", "VERBS"}}
	for _, r := range answer.Status.ResourceRules {
		var resources []string
		for _, g := range r.APIGroups {
			for _, res := range r.Resources {
				if g != "" && g != "*" {
					res += "." + g
				}
				resources = append(resources, res)
			}
		}
		names := "[" + strings.Join(r.ResourceNames, " ") + "]"
		rows = append(rows, []string{strings.Join(resources, ","), names, "[" + strings.Join(r.Verbs, " ") + "]"})
	}
	for _, r := range answer.Status.NonResourceRules {
		rows = append(rows, []string{strings.Join(r.NonResourceURLs, ","), "[]", "[" + strings.Join(r.Verbs, " ") + "]"})
	}
	text := strings.Join(alignedRows(rows), "\n")
	if answer.Status.Incomplete {
		text += "\n\n(not all rules are known: " + answer.Status.EvaluationError + ")"
	}
	return text, nil
}
